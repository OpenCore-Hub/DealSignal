#!/usr/bin/env python3
"""
Copy the template library to another project and perform basic
project-specific substitutions.

Usage (from the repository that contains this package):
    python3 docs/templates/scripts/copy-templates-to-project.py \
        --target /path/to/your-project/docs/templates \
        --product-name "Your Product" \
        --project-prefix YP

After copying, review the reported residual placeholders before committing.
"""

import argparse
import re
import shutil
import sys
from pathlib import Path

# This script lives in <package-root>/scripts/; the package root is the directory
# that contains README.md, templates-manifest.yaml, examples/, scripts/, etc.
ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT

# File paths relative to SOURCE that should be copied.
FILES_TO_COPY = [
    "README.md",
    "templates-manifest.yaml",
    "templates-schema.json",
    "pyproject.toml",
]

DIRS_TO_COPY = [
    "examples",
    "scripts",
    # NOTE: archive/ is intentionally excluded from distribution copies.
    # Historical templates remain available in the source repository only.
]

# Template files use {占位符} syntax with canonical semantics.
# Each key below maps to a CLI argument. Defaults cascade to --product-name
# so a single-product project only needs to provide one value.
DEFAULT_SUBSTITUTIONS = {
    "{产品名}": None,      # --product-name
    "{模块名}": None,      # --module-name  (defaults to product-name)
    "{功能名称}": None,    # --feature-name (defaults to product-name)
    "{项目名称}": None,    # --project-name (defaults to product-name)
    "{品牌名}": None,      # --brand-name   (defaults to product-name)
    "{系统名}": None,      # --system-name  (defaults to product-name)
    "{公司名}": None,      # --company-name (defaults to product-name)
    "{组织标识}": None,    # derived from --company-name slug
    "{项目前缀}": None,    # --project-prefix
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Copy the general-purpose template library to another project."
    )
    parser.add_argument(
        "--target",
        required=True,
        help="Destination directory (e.g. /path/to/project/docs/templates)",
    )
    parser.add_argument(
        "--product-name",
        default="Your Product",
        help="Product name to replace '{产品名}' placeholders",
    )
    parser.add_argument(
        "--module-name",
        default=None,
        help="Module name to replace '{模块名}' (defaults to --product-name)",
    )
    parser.add_argument(
        "--feature-name",
        default=None,
        help="Feature name to replace '{功能名称}' (defaults to --product-name)",
    )
    parser.add_argument(
        "--project-name",
        default=None,
        help="Project name to replace '{项目名称}' (defaults to --product-name)",
    )
    parser.add_argument(
        "--brand-name",
        default=None,
        help="Brand name to replace '{品牌名}' (defaults to --product-name)",
    )
    parser.add_argument(
        "--system-name",
        default=None,
        help="System name to replace '{系统名}' (defaults to --product-name)",
    )
    parser.add_argument(
        "--company-name",
        default=None,
        help="Company name to replace '{公司名}' (defaults to --product-name)",
    )
    parser.add_argument(
        "--org-identifier",
        default=None,
        help="Organization identifier/slug to replace '{组织标识}' (defaults to slug of --company-name)",
    )
    parser.add_argument(
        "--project-prefix",
        default="YP",
        help="Project/issue ID prefix to replace '{项目前缀}' (e.g. YP)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print what would be copied without modifying the target",
    )
    parser.add_argument(
        "--report-json",
        action="store_true",
        help="Output a structured JSON summary to stdout",
    )
    parser.add_argument(
        "--skip-examples",
        action="store_true",
        help="Do not copy the examples/ directory",
    )
    parser.add_argument(
        "--select-domain",
        action="append",
        default=None,
        help=(
            "Copy only examples for a given domain. Supported values: "
            "crm, ecommerce, saas-billing, generic. "
            "'generic' selects examples without a domain suffix. Can be repeated."
        ),
    )
    return parser.parse_args()


def build_substitutions(args: argparse.Namespace) -> dict[str, str]:
    product = args.product_name
    module = args.module_name or product
    feature = args.feature_name or product
    project = args.project_name or product
    brand = args.brand_name or product
    system = args.system_name or product
    company = args.company_name or product
    prefix = args.project_prefix
    # Derive a URL/package friendly org identifier from company name.
    org_id = args.org_identifier or re.sub(r"[^a-z0-9]+", "-", company.lower()).strip("-")
    return {
        "{产品名}": product,
        "{模块名}": module,
        "{功能名称}": feature,
        "{项目名称}": project,
        "{品牌名}": brand,
        "{系统名}": system,
        "{公司名}": company,
        "{组织标识}": org_id,
        "{项目前缀}": prefix,
    }


# Domains that produce a filename suffix like `*-crm-example*.md`.
KNOWN_DOMAIN_SUFFIXES = {"crm", "ecommerce", "saas-billing"}


def _example_matches_domain(example_path: Path, domains: list[str] | None) -> bool:
    """Return True if an example file matches any of the selected domains."""
    if not domains:
        return True
    stem = example_path.stem.lower()
    selected = {d.lower() for d in domains}

    if "generic" in selected:
        # A generic example has no known domain suffix.
        if not any(f"-{domain}-example" in stem for domain in KNOWN_DOMAIN_SUFFIXES):
            return True

    return any(f"-{domain}-example" in stem for domain in selected)


def _copy_examples(
    src_dir: Path, dst_dir: Path, select_domains: list[str] | None, dry_run: bool
) -> list[Path]:
    """Copy examples, optionally filtered by domain."""
    copied: list[Path] = []
    if not dry_run:
        if dst_dir.exists():
            shutil.rmtree(dst_dir)
        dst_dir.mkdir(parents=True, exist_ok=True)
    for src_file in sorted(src_dir.iterdir()):
        if not src_file.is_file() or src_file.suffix != ".md":
            continue
        if not _example_matches_domain(src_file, select_domains):
            continue
        dst = dst_dir / src_file.name
        if dry_run:
            print(f"[dry-run] would copy example {src_file.relative_to(ROOT)} -> {dst}")
        else:
            shutil.copy2(src_file, dst)
        copied.append(dst)
    return copied


def copy_template_files(
    source: Path,
    target: Path,
    dry_run: bool,
    skip_examples: bool = False,
    select_domains: list[str] | None = None,
) -> list[Path]:
    copied: list[Path] = []

    # Copy markdown / yaml template files at the root of SOURCE.
    # Skip internal review/audit artifacts that should not be distributed.
    for src_file in sorted(source.iterdir()):
        if not src_file.is_file():
            continue
        if src_file.suffix not in {".md", ".yaml", ".json", ".toml"}:
            continue
        if src_file.name.startswith("REVIEW-"):
            continue
        dst = target / src_file.name
        if dry_run:
            print(f"[dry-run] would copy {src_file.relative_to(ROOT)} -> {dst}")
        else:
            target.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src_file, dst)
        copied.append(dst)

    # Copy selected subdirectories
    for name in DIRS_TO_COPY:
        src_dir = source / name
        dst_dir = target / name
        if not src_dir.exists():
            continue
        if name == "examples":
            if skip_examples:
                if dry_run:
                    print(f"[dry-run] would skip examples/")
                continue
            copied.extend(_copy_examples(src_dir, dst_dir, select_domains, dry_run))
            continue
        if dry_run:
            print(f"[dry-run] would copy tree {src_dir.relative_to(ROOT)} -> {dst_dir}")
        else:
            if dst_dir.exists():
                shutil.rmtree(dst_dir)
            shutil.copytree(src_dir, dst_dir)
        copied.append(dst_dir)

    return copied


def apply_substitutions(target: Path, substitutions: dict[str, str]) -> list[Path]:
    changed: list[Path] = []
    for path in sorted(target.rglob("*")):
        if not path.is_file():
            continue
        if path.suffix not in {".md", ".yaml", ".json", ".py", ".toml"}:
            continue
        # Do not substitute inside the copied scripts directory: the copy script
        # itself contains literal placeholder names (e.g. "{产品名}") as part of
        # its internal dictionary and must remain generic so the downstream team
        # can re-run it with their own values.
        if "scripts" in path.relative_to(target).parts:
            continue
        text = path.read_text(encoding="utf-8")
        new_text = text
        for old, new in substitutions.items():
            new_text = new_text.replace(old, new)
        # Collapse any accidental double braces that may have been introduced when
        # the project prefix placeholder is adjacent to other braces.
        new_text = new_text.replace("{{项目前缀}}", "{项目前缀}")
        if new_text != text:
            path.write_text(new_text, encoding="utf-8")
            changed.append(path)
    return changed


def find_residual_placeholders(target: Path) -> list[tuple[Path, int, str]]:
    """Find canonical placeholders that were not substituted during copy.

    Only flags the canonical substitution keys (the nine CLI-mapped placeholders
    plus {项目前缀}) so the report is actionable. Common template instructions
    such as `{YYYY-MM-DD}`, `{姓名}`, status enums, option lists, path variables
    and example snippets are intentionally ignored.

    Skips code blocks and YAML front matter.
    """
    canonical_keys = list(DEFAULT_SUBSTITUTIONS.keys()) + ["{项目前缀}"]
    # Build a regex for each key that tolerates optional whitespace inside braces.
    placeholder_patterns = [
        re.compile(r"\{\s*" + re.escape(key.strip("{}")) + r"\s*\}")
        for key in canonical_keys
    ]
    code_fence_re = re.compile(r"^\s*```")
    frontmatter_delim_re = re.compile(r"^---\s*$")
    issues = []
    for path in sorted(target.rglob("*")):
        if not path.is_file():
            continue
        if path.suffix not in {".md", ".yaml", ".yml", ".json", ".py", ".toml"}:
            continue
        # The copy script intentionally contains literal placeholder names as
        # code; skip the scripts directory to avoid false positives.
        if "scripts" in path.relative_to(target).parts:
            continue
        in_code_block = False
        in_frontmatter = False
        frontmatter_seen = False
        for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            # Toggle front matter state (only at the top of Markdown files).
            if path.suffix == ".md" and frontmatter_delim_re.match(line):
                if not frontmatter_seen:
                    in_frontmatter = True
                    frontmatter_seen = True
                elif in_frontmatter:
                    in_frontmatter = False
                continue
            if in_frontmatter:
                continue
            # Toggle code block state.
            if code_fence_re.match(line):
                in_code_block = not in_code_block
                continue
            if in_code_block:
                continue
            if any(pattern.search(line) for pattern in placeholder_patterns):
                issues.append((path, line_no, line.strip()))
    return issues


# Tokens that may leak project-specific context into the generic templates.
# These are reported as warnings so downstream teams can review them.
PROJECT_SPECIFIC_TOKENS = [
    "Secure Link",
    "secure-link",
    "secure_link",
    "Acme",
    "acmeshare",
]


def find_project_specific_tokens(target: Path) -> list[tuple[Path, int, str, str]]:
    """Find lines that still contain tokens from the source project's domain.

    Excludes the copied scripts directory because it contains validation code
    that references these tokens as part of its checks.
    """
    findings = []
    for path in sorted(target.rglob("*")):
        if not path.is_file():
            continue
        if path.suffix not in {".md", ".yaml", ".yml", ".json", ".py", ".toml"}:
            continue
        # Skip copied validation scripts.
        if "scripts" in path.relative_to(target).parts:
            continue
        text = path.read_text(encoding="utf-8")
        for line_no, line in enumerate(text.splitlines(), start=1):
            for token in PROJECT_SPECIFIC_TOKENS:
                if token in line:
                    findings.append((path, line_no, line.strip(), token))
                    break
    return findings


def build_report(
    target: Path,
    copied: list[Path],
    changed: list[Path],
    issues: list[tuple[Path, int, str]],
    tokens: list[tuple[Path, int, str, str]],
) -> dict:
    from collections import Counter

    file_counts = Counter(str(path.relative_to(target)) for path, _, _ in issues)
    token_counts = Counter(token for _, _, _, token in tokens)
    return {
        "target": str(target),
        "copied_count": len(copied),
        "substituted_count": len(changed),
        "residual_placeholder_lines": len(issues),
        "residual_files": dict(file_counts.most_common()),
        "project_specific_token_hits": dict(token_counts.most_common()),
        "samples": [
            {
                "file": str(path.relative_to(target)),
                "line": line_no,
                "content": line[:200],
            }
            for path, line_no, line in issues[:10]
        ],
    }


def main() -> int:
    args = parse_args()
    target = Path(args.target).expanduser().resolve()
    substitutions = build_substitutions(args)

    if args.dry_run:
        print(f"[dry-run] Source: {SOURCE}")
        print(f"[dry-run] Target: {target}")
        print(f"[dry-run] Substitutions: {substitutions}")

    copied = copy_template_files(
        SOURCE,
        target,
        args.dry_run,
        skip_examples=args.skip_examples,
        select_domains=args.select_domain,
    )

    if not args.dry_run:
        changed = apply_substitutions(target, substitutions)
        issues = find_residual_placeholders(target)
        tokens = find_project_specific_tokens(target)

        if args.report_json:
            import json

            report = build_report(target, copied, changed, issues, tokens)
            print(json.dumps(report, ensure_ascii=False, indent=2))
            return 0

        print(f"Copied {len(copied)} file/dir(s) to {target}")
        if changed:
            print(f"Applied substitutions to {len(changed)} file(s)")

        if issues:
            from collections import Counter

            file_counts = Counter(str(path.relative_to(target)) for path, _, _ in issues)
            print(f"\nWARNING: {len(issues)} residual placeholder line(s) found across {len(file_counts)} file(s).")
            print("Top files by residual count:")
            for rel, count in file_counts.most_common(10):
                print(f"  {rel}: {count} line(s)")
            print("\nSample residual placeholders:")
            for path, line_no, line in issues[:10]:
                rel = path.relative_to(target)
                print(f"  {rel}:{line_no}: {line}")
            if len(issues) > 10:
                print(f"  ... and {len(issues) - 10} more line(s)")
            print("\nPlease review and replace these placeholders before using the templates.")
        else:
            print("No obvious residual placeholders found.")

        if tokens:
            from collections import Counter

            token_counts = Counter(token for _, _, _, token in tokens)
            print(f"\nNOTE: {len(tokens)} line(s) contain project-specific token(s) from the source templates.")
            print("Review whether these should be replaced for your project:")
            for token, count in token_counts.most_common():
                print(f"  '{token}': {count} occurrence(s)")

        print(
            f"\nSummary: {len(copied)} file/dir(s) copied, "
            f"{len(changed)} file(s) substituted, "
            f"{len(issues)} residual placeholder line(s)."
        )
        print("\nNext steps:")
        print(f"  1. cd {target.parent.parent}")
        print("  2. Review and replace any remaining {占位符}")
        print("  3. Run: python3 docs/templates/scripts/validate_templates.py")
        print("  4. Run: openapi-spec-validator docs/templates/openapi-template-v1.yaml")

    return 0


if __name__ == "__main__":
    sys.exit(main())
