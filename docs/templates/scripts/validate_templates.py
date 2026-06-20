#!/usr/bin/env python3
"""
Validate docs/templates/templates-manifest.yaml against the actual template files.

Checks:
1. Manifest parses and matches templates-schema.json.
2. Every template listed in the manifest exists on disk.
3. Templates with YAML front matter contain all required_frontmatter_fields.
4. Front matter values are valid per JSON Schema (when jsonschema is available).
5. The 'v{N}' in the filename matches the '模板版本' value in the header.
6. No unresolved TODO / FIXME markers remain (excluding references in checklist text).
7. Examples in docs/templates/examples/ do not contain unsubstituted placeholders.
8. Cross-references in related_templates point to valid manifest IDs.
9. Bidirectional related_templates references are encouraged (warning if missing).

Exit codes:
- 0: all checks passed
- non-zero: one or more checks failed
"""

import argparse
import json
import re
import sys
from pathlib import Path

# The script can run from either:
#   <project-root>/scripts/validate-templates.py
#   <project-root>/docs/templates/scripts/validate-templates.py (package layout)
_SCRIPT_DIR = Path(__file__).resolve().parent
if (_SCRIPT_DIR / "templates-manifest.yaml").exists():
    TEMPLATES_DIR = _SCRIPT_DIR
elif (_SCRIPT_DIR.parent / "templates-manifest.yaml").exists():
    TEMPLATES_DIR = _SCRIPT_DIR.parent
else:
    TEMPLATES_DIR = _SCRIPT_DIR.parent.parent / "docs" / "templates"

MANIFEST_FILE = TEMPLATES_DIR / "templates-manifest.yaml"
SCHEMA_FILE = TEMPLATES_DIR / "templates-schema.json"
EXAMPLES_DIR = TEMPLATES_DIR / "examples"

# Optional dependencies: PyYAML and jsonschema. If missing, we degrade gracefully.
try:
    import yaml  # type: ignore
except Exception:  # pragma: no cover
    yaml = None  # type: ignore

try:
    import jsonschema  # type: ignore
except Exception:  # pragma: no cover
    jsonschema = None  # type: ignore


# Regex for an explicit TODO/FIXME marker: followed by ':' or '-' (with optional whitespace),
# or at the very end of the line. This avoids flagging normal mentions such as
# "清除所有 TODO/FIXME 标记" in template instructions.
TODO_FIXME_RE = re.compile(
    r"(^|\s|\W)(TODO|FIXME)\s*([:\-]|$)",
    re.IGNORECASE,
)

# Template-style placeholder patterns. These are expected in templates (they ARE
# templates), but must not remain in examples.
# Matches braces containing CJK characters, placeholder keywords, or option lists.
TEMPLATE_PLACEHOLDER_RE = re.compile(
    r"\{[^{}\n]*(?:"
    r"[\u4e00-\u9fff]|"
    r"XXX|xxxx|YYYY|NNN|"
    r"占位符|姓名|产品|模块|功能|项目|版本|状态|负责人|编写人|"
    r"\s+/\s+"
    r")[^{}\n]*\}",
    re.IGNORECASE,
)


# Map example filenames to template types for front matter schema validation.
EXAMPLE_TYPE_MAP = {
    "ADR-crm-example-v1.0.0": "adr",
    "ADR-ecommerce-example-v1.0.0": "adr",
    "ADR-saas-billing-example-v1.0.0": "adr",
    "AGENT-TASK-example-v1.0.0": "agent-task",
    "API-SPEC-crm-example-v1.0.0": "api-spec",
    "API-SPEC-example-v1.0.0": "api-spec",
    "API-SPEC-saas-billing-example-v1.0.0": "api-spec",
    "DATABASE-MODEL-crm-example-v1.0.0": "database",
    "DATABASE-MODEL-example-v1.0.0": "database",
    "DATABASE-MODEL-saas-billing-example-v1.0.0": "database",
    "IMPLEMENTATION-PLAN-example-v1.0.0": "plan",
    "PRD-crm-example-v1.0.0": "prd",
    "PRD-ecommerce-example-v1.0.0": "prd",
    "PRD-example-v1.0.0": "prd",
    "PRD-saas-billing-example-v1.0.0": "prd",
    "QA-TEST-PLAN-crm-example-v1.0.0": "test-plan",
    "QA-TEST-PLAN-example-v1.0.0": "test-plan",
    "QA-TEST-PLAN-saas-billing-example-v1.0.0": "test-plan",
    "TDD-crm-example-v1.0.0": "tdd",
    "TDD-example-v1.0.0": "tdd",
    "TDD-saas-billing-example-v1.0.0": "tdd",
}


def parse_value(value: str):
    """Parse a simple YAML scalar or inline list (legacy fallback)."""
    value = value.strip()
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        if not inner:
            return []
        return [
            item.strip().strip('"').strip("'")
            for item in inner.split(",")
            if item.strip()
        ]
    value = value.strip('"').strip("'")
    return value


def parse_manifest_legacy(path: Path) -> dict:
    """Parse the minimal YAML manifest format used by templates-manifest.yaml."""
    with open(path, "r", encoding="utf-8") as f:
        content = f.read()

    manifest = {"version": None, "templates": []}
    current = None
    in_templates = False

    for raw_line in content.splitlines():
        line = raw_line.rstrip("\n")
        stripped = line.strip()

        if not stripped or stripped.startswith("#"):
            continue

        if not in_templates:
            if stripped == "templates:":
                in_templates = True
                continue
            if ":" in stripped:
                key, val = stripped.split(":", 1)
                if key.strip() == "version":
                    manifest["version"] = parse_value(val)
            continue

        # Inside templates list
        if stripped.startswith("- "):
            if current:
                manifest["templates"].append(current)
            current = {}
            rest = stripped[2:].strip()
            if ":" in rest:
                key, val = rest.split(":", 1)
                current[key.strip()] = parse_value(val)
        elif current is not None and line.startswith(" ") and ":" in stripped:
            key, val = stripped.split(":", 1)
            current[key.strip()] = parse_value(val)
        else:
            # Left the templates list
            in_templates = False
            if current:
                manifest["templates"].append(current)
                current = None

    if current:
        manifest["templates"].append(current)

    return manifest


def load_manifest(path: Path) -> dict:
    if yaml is not None:
        with open(path, "r", encoding="utf-8") as f:
            return yaml.safe_load(f)
    return parse_manifest_legacy(path)


def load_schema(path: Path) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def extract_filename_version(filename: str) -> str | None:
    """Extract the template version number from the filename, e.g. PRD-template-v2.md -> 2."""
    match = re.search(r"-template-v(\d+)(?:\.\d+)*", filename)
    if match:
        return match.group(1)
    return None


def extract_header_version(content: str) -> str | None:
    """Extract the '模板版本' value from the document header, e.g. 'v2' -> 2."""
    match = re.search(r"模板版本[\s*]*[：:]\s*[`\"']?v(\d+)[`\"']?", content)
    if match:
        return match.group(1)
    return None


def has_chinese(text: str) -> bool:
    """Return True if the text contains any CJK character."""
    return bool(re.search(r"[\u4e00-\u9fff]", text))


def is_checklist_line(line: str) -> bool:
    """Return True if the line is a markdown checkbox item or a checklist table row."""
    stripped = line.strip()
    if re.match(r"^[-*]\s*\[[ xX]\]", stripped):
        return True
    # Table rows that contain a checkbox cell or checklist keywords
    if "|" in stripped and (
        re.search(r"\[\s*[ xX]\s*\]", stripped)
        or "检查清单" in stripped
        or "checklist" in stripped.lower()
    ):
        return True
    return False


def find_unresolved_todos(content: str) -> list[tuple[int, str]]:
    """Find lines that contain unresolved TODO/FIXME markers."""
    issues = []
    for line_no, line in enumerate(content.splitlines(), start=1):
        if not re.search(r"TODO|FIXME", line, re.IGNORECASE):
            continue

        if is_checklist_line(line):
            continue

        # Heuristic: skip Chinese instruction lines where TODO/FIXME is mentioned
        # but not used as a marker (i.e. not followed by ':' or '-').
        if has_chinese(line):
            match = re.search(r"(TODO|FIXME)\s*([:\-]?)", line, re.IGNORECASE)
            if match and match.group(2) not in (":", "-"):
                continue

        if TODO_FIXME_RE.search(line):
            issues.append((line_no, line.strip()))

    return issues


def extract_frontmatter(content: str, is_yaml_file: bool = False) -> tuple[dict | None, str]:
    """Extract YAML front matter from markdown content. Returns (frontmatter, rest)."""
    if is_yaml_file or not content.startswith("---\n"):
        return None, content
    parts = content.split("---\n", 2)
    if len(parts) < 3:
        return None, content
    fm_text = parts[1]
    rest = parts[2]
    if yaml is not None:
        try:
            fm = yaml.safe_load(fm_text)
            if not isinstance(fm, dict):
                return None, content
            return fm, rest
        except Exception as exc:
            return {"__parse_error__": str(exc)}, rest
    return None, content


def validate_frontmatter_against_schema(
    frontmatter: dict, template_type: str, schema: dict
) -> list[str]:
    """Validate front matter against the JSON Schema for its type."""
    if jsonschema is None:
        return []
    type_schema = schema.get("$defs", {}).get("frontmatterSchemas", {}).get("properties", {}).get(template_type)
    if type_schema is None:
        type_schema = schema.get("$defs", {}).get("frontmatterSchemas", {}).get("properties", {}).get("default")
    if type_schema is None:
        return []
    # Copy $defs into the subschema so internal $refs resolve.
    subschema = dict(type_schema)
    subschema["$defs"] = schema.get("$defs", {})
    try:
        jsonschema.validate(frontmatter, subschema)
    except jsonschema.ValidationError as exc:
        return [f"front matter schema error: {exc.message} at {list(exc.path)}"]
    return []


def validate_required_fields(frontmatter: dict | None, required: list[str]) -> list[str]:
    """Ensure required front matter fields are present and non-empty."""
    if frontmatter is None:
        if required:
            return [f"missing front matter block (required fields: {required})"]
        return []
    issues = []
    for field in required:
        value = frontmatter.get(field)
        if value is None or value == "" or value == []:
            issues.append(f"missing or empty front matter field '{field}'")
    return issues


def find_template_placeholders(content: str) -> list[tuple[int, str]]:
    """Return non-code lines containing template-style placeholders."""
    issues = []
    in_code_block = False
    code_fence_re = re.compile(r"^\s*```")
    for line_no, line in enumerate(content.splitlines(), start=1):
        if code_fence_re.match(line):
            in_code_block = not in_code_block
            continue
        if in_code_block:
            continue
        if TEMPLATE_PLACEHOLDER_RE.search(line):
            issues.append((line_no, line.strip()))
    return issues


def validate_readme_table(readme_path: Path, templates: list[dict]) -> list[str]:
    """Ensure every manifest .md template is listed in the README table."""
    if not readme_path.exists():
        return ["README.md not found"]
    readme = readme_path.read_text(encoding="utf-8")
    table_files = set()
    for line in readme.splitlines():
        if not line.strip().startswith("|"):
            continue
        for token in re.findall(r"`([^`]+\.md)`", line):
            table_files.add(token)
    failures = []
    for entry in templates:
        file_name = entry.get("file")
        if not file_name or not file_name.endswith(".md"):
            continue
        if file_name not in table_files:
            failures.append(
                f"README.md template table is missing manifest entry '{file_name}'"
            )
    return failures


def find_nested_braces(content: str) -> list[tuple[int, str]]:
    """Warn about nested braces like {foo {bar} baz} which evade simple placeholder regex."""
    issues = []
    for line_no, line in enumerate(content.splitlines(), start=1):
        # Look for a '{', then another '{' before the matching '}'
        depth = 0
        for ch in line:
            if ch == "{":
                depth += 1
                if depth > 1:
                    issues.append((line_no, line.strip()))
                    break
            elif ch == "}":
                depth = max(0, depth - 1)
    return issues


def check_example_naming_consistency(examples_dir: Path) -> list[str]:
    """Warn if examples mix snake_case naming styles for the same conceptual entity."""
    warnings = []
    if not examples_dir.exists():
        return warnings
    for example_path in sorted(examples_dir.iterdir()):
        if example_path.is_file() and example_path.suffix == ".md":
            content = example_path.read_text(encoding="utf-8")
            if "SMART_LINK" in content and "secure_link" in content:
                warnings.append(
                    f"{example_path.relative_to(examples_dir.parent)}: mixes SMART_LINK and secure_link naming"
                )
    return warnings


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate the general-purpose document template library")
    parser.add_argument("--verbose", action="store_true", help="Print all warning details")
    parser.add_argument("--strict", action="store_true", help="Treat warnings as failures")
    args = parser.parse_args(argv)

    if not MANIFEST_FILE.exists():
        print(f"FAIL: Manifest not found: {MANIFEST_FILE}")
        return 1

    if not SCHEMA_FILE.exists():
        print(f"FAIL: Schema not found: {SCHEMA_FILE}")
        return 1

    try:
        manifest = load_manifest(MANIFEST_FILE)
    except Exception as exc:
        print(f"FAIL: Could not parse manifest: {exc}")
        return 1

    try:
        schema = load_schema(SCHEMA_FILE)
    except Exception as exc:
        print(f"FAIL: Could not parse schema: {exc}")
        return 1

    templates = manifest.get("templates", [])
    if not templates:
        print("FAIL: No templates found in manifest")
        return 1

    # Validate manifest against schema
    failures = []
    if jsonschema is not None:
        try:
            jsonschema.validate(manifest, schema)
        except jsonschema.ValidationError as exc:
            failures.append(f"manifest schema error: {exc.message} at {list(exc.path)}")
    else:
        print("WARNING: jsonschema not installed; skipping JSON Schema validation")

    if yaml is None:
        print("WARNING: PyYAML not installed; front matter parsing is limited")

    failures.extend(validate_readme_table(TEMPLATES_DIR / "README.md", templates))

    id_to_entry = {t.get("id"): t for t in templates if t.get("id")}
    checked = 0

    for entry in templates:
        file_name = entry.get("file")
        tmpl_id = entry.get("id") or file_name

        if not file_name:
            failures.append(f"{tmpl_id}: missing 'file' field in manifest")
            continue

        file_path = TEMPLATES_DIR / file_name
        checked += 1

        if not file_path.exists():
            failures.append(f"{file_name}: file not found")
            continue

        try:
            content = file_path.read_text(encoding="utf-8")
        except Exception as exc:
            failures.append(f"{file_name}: could not read file ({exc})")
            continue

        is_yaml = file_name.endswith(".yaml") or file_name.endswith(".yml")

        # Check required manifest fields
        for required_field in ("id", "file", "type", "stage", "maturity", "required_frontmatter_fields"):
            value = entry.get(required_field)
            if value is None:
                failures.append(
                    f"{file_name}: missing manifest field '{required_field}'"
                )

        # Extract and validate front matter for markdown templates
        if not is_yaml:
            frontmatter, _ = extract_frontmatter(content)
            required_fm = entry.get("required_frontmatter_fields") or []
            failures.extend(
                f"{file_name}: {msg}" for msg in validate_required_fields(frontmatter, required_fm)
            )
            if frontmatter and not frontmatter.get("__parse_error__"):
                failures.extend(
                    f"{file_name}: {msg}"
                    for msg in validate_frontmatter_against_schema(
                        frontmatter, entry.get("type", ""), schema
                    )
                )
            elif frontmatter and frontmatter.get("__parse_error__"):
                failures.append(f"{file_name}: could not parse YAML front matter: {frontmatter['__parse_error__']}")

        # Check template version field in header
        header_version = extract_header_version(content)
        if not header_version:
            failures.append(f"{file_name}: missing '模板版本' field in header")
        else:
            filename_version = extract_filename_version(file_name)
            if filename_version and header_version != filename_version:
                failures.append(
                    f"{file_name}: filename version v{filename_version} does not match "
                    f"header version v{header_version}"
                )

        # Check unresolved TODO/FIXME markers
        for line_no, line in find_unresolved_todos(content):
            failures.append(f"{file_name}:{line_no}: unresolved TODO/FIXME: {line}")

        # Validate related_templates cross-references
        related = entry.get("related_templates") or []
        for related_id in related:
            if related_id not in id_to_entry:
                failures.append(
                    f"{file_name}: related_templates references unknown id '{related_id}'"
                )

    # Nested-brace warnings for templates
    warnings = []
    for entry in templates:
        file_name = entry.get("file")
        if not file_name:
            continue
        file_path = TEMPLATES_DIR / file_name
        if not file_path.exists():
            continue
        content = file_path.read_text(encoding="utf-8")
        for line_no, line in find_nested_braces(content):
            warnings.append(
                f"{file_name}:{line_no}: nested braces may evade placeholder detection: {line}"
            )

    # Bidirectional related_templates warnings
    for entry in templates:
        tmpl_id = entry.get("id")
        related = set(entry.get("related_templates") or [])
        for other_id in related:
            other = id_to_entry.get(other_id)
            if other and tmpl_id not in (other.get("related_templates") or []):
                warnings.append(
                    f"{entry.get('file')}: related_templates '{other_id}' does not reference back '{tmpl_id}'"
                )

    # Validate examples: no placeholder residue and valid front matter
    if EXAMPLES_DIR.exists():
        for example_path in sorted(EXAMPLES_DIR.iterdir()):
            if example_path.is_file() and example_path.suffix == ".md":
                content = example_path.read_text(encoding="utf-8")
                rel = example_path.relative_to(TEMPLATES_DIR)
                for line_no, line in find_template_placeholders(content):
                    failures.append(
                        f"{rel}:{line_no}: unsubstituted placeholder: {line}"
                    )
                for line_no, line in find_nested_braces(content):
                    warnings.append(
                        f"{rel}:{line_no}: nested braces may evade placeholder detection: {line}"
                    )
                example_type = EXAMPLE_TYPE_MAP.get(example_path.stem)
                if example_type and yaml is not None:
                    fm, _ = extract_frontmatter(content)
                    if fm is None:
                        failures.append(f"{rel}: missing YAML front matter")
                    elif fm.get("__parse_error__"):
                        failures.append(
                            f"{rel}: could not parse YAML front matter: {fm['__parse_error__']}"
                        )
                    else:
                        failures.extend(
                            f"{rel}: {msg}"
                            for msg in validate_frontmatter_against_schema(
                                fm, example_type, schema
                            )
                        )

    warnings.extend(check_example_naming_consistency(EXAMPLES_DIR))

    print(f"Manifest version: {manifest.get('version') or 'not set'}")
    print(f"Schema file: {SCHEMA_FILE}")
    print(f"Checked {checked} template(s) from {MANIFEST_FILE}")

    if warnings:
        print(f"\nWARNINGS: {len(warnings)} issue(s) found")
        if args.verbose:
            for warning in warnings:
                print(f"  - {warning}")
        else:
            print("  (run with --verbose to see all)")
        if args.strict:
            failures.extend(warnings)

    if failures:
        print("\nFAILURES:")
        for failure in failures:
            print(f"  - {failure}")
        print(f"\nResult: FAILED ({len(failures)} issue(s))")
        return 1

    print("\nResult: PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
