import {
  type CSSProperties,
  type ComponentProps,
  type ReactNode,
  useEffect,
  useRef,
  useState,
} from "react";
import { motion, useReducedMotion as useMotionReducedMotion } from "motion/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { cn } from "@/lib/utils";

export type AuthRailSignal = {
  index: string;
  title: string;
  body: string;
};

export type AuthRailCopy = {
  brand: string;
  folio: string;
  kicker: string;
  headline: string;
  lede: string;
  signals: AuthRailSignal[];
  footnote: string;
};

export function railFromNamespace(
  t: (key: string) => string,
  root: string,
): AuthRailCopy {
  return {
    brand: t(`${root}.brand`),
    folio: t(`${root}.folio`),
    kicker: t(`${root}.kicker`),
    headline: t(`${root}.headline`),
    lede: t(`${root}.lede`),
    footnote: t(`${root}.footnote`),
    signals: [0, 1, 2].map((i) => ({
      index: t(`${root}.signals.${i}.index`),
      title: t(`${root}.signals.${i}.title`),
      body: t(`${root}.signals.${i}.body`),
    })),
  };
}

type AuthStageProps = {
  rail: AuthRailCopy;
  kicker?: string;
  title: string;
  lede?: string;
  notice?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
};

const EASE = [0.16, 1, 0.3, 1] as const;
const ORBIT_SECONDS = [22, 34, 48];

function AuthCosmos({ reduced }: { reduced: boolean }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const stars = Array.from({ length: 70 }, () => ({
      x: Math.random(),
      y: Math.random(),
      r: Math.random() * 1.05 + 0.2,
      a: Math.random() * 0.28 + 0.08,
      tw: Math.random() * Math.PI * 2,
      sp: 0.4 + Math.random() * 1.4,
    }));

    let streak = { x: -1, y: 0.2, len: 0, life: 0 };
    let frame = 0;
    let raf = 0;

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const { clientWidth, clientHeight } = canvas;
      canvas.width = Math.max(1, Math.floor(clientWidth * dpr));
      canvas.height = Math.max(1, Math.floor(clientHeight * dpr));
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };

    const draw = () => {
      const w = canvas.clientWidth;
      const h = canvas.clientHeight;
      ctx.clearRect(0, 0, w, h);
      for (const star of stars) {
        const twinkle = reduced ? 1 : 0.55 + Math.sin(frame * 0.018 * star.sp + star.tw) * 0.45;
        ctx.beginPath();
        ctx.fillStyle = `rgba(15, 23, 42, ${star.a * twinkle})`;
        ctx.arc(star.x * w, star.y * h, star.r, 0, Math.PI * 2);
        ctx.fill();
      }
      if (!reduced && streak.life > 0) {
        ctx.strokeStyle = `rgba(226, 59, 44, ${streak.life * 0.45})`;
        ctx.lineWidth = 1.1;
        ctx.beginPath();
        ctx.moveTo(streak.x * w, streak.y * h);
        ctx.lineTo((streak.x + streak.len) * w, (streak.y + streak.len * 0.28) * h);
        ctx.stroke();
        streak.x += 0.012;
        streak.y += 0.0034;
        streak.life -= 0.02;
      } else if (!reduced && frame % 280 === 40) {
        streak = { x: Math.random() * 0.5, y: Math.random() * 0.4, len: 0.08 + Math.random() * 0.08, life: 0.85 };
      }
      frame += 1;
      if (!reduced) raf = window.requestAnimationFrame(draw);
    };

    resize();
    draw();
    window.addEventListener("resize", resize);
    return () => {
      window.removeEventListener("resize", resize);
      window.cancelAnimationFrame(raf);
    };
  }, [reduced]);

  return <canvas ref={canvasRef} className="auth-stars" aria-hidden />;
}

export function AuthStage({ rail, kicker, title, lede, notice, footer, children }: AuthStageProps) {
  const reducedMotion = useReducedMotion() || useMotionReducedMotion();
  const [active, setActive] = useState(0);
  const [held, setHeld] = useState(false);
  const instant = reducedMotion ? { duration: 0 } : { duration: 0.85, ease: EASE };

  useEffect(() => {
    if (reducedMotion || held) return;
    const id = window.setInterval(() => {
      setActive((current) => (current + 1) % 3);
    }, 3200);
    return () => window.clearInterval(id);
  }, [reducedMotion, held]);

  return (
    <div className="auth-stage">
      <div className="auth-field-bg" aria-hidden>
        <AuthCosmos reduced={reducedMotion} />
        <div className="auth-nebula" />
      </div>

      <div className="auth-salon">
        <motion.p
          className="auth-brand"
          initial={reducedMotion ? false : { opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={instant}
        >
          {rail.brand}
        </motion.p>

        <div className="auth-compose">
          <section className="auth-copy">
            <div className="auth-system">
              <div className="auth-planet" aria-hidden>
                <span className="auth-ring" />
                <span className="auth-planet-atm" />
                <span className="auth-planet-body">
                  <span className="auth-planet-tex" />
                  <span className="auth-planet-light" />
                </span>
              </div>
              {rail.signals.map((signal, index) => (
                <div
                  key={signal.index}
                  className="auth-orbit"
                  style={
                    {
                      "--auth-orbit-s": `${ORBIT_SECONDS[index]}s`,
                      "--auth-orbit-i": `${index}`,
                    } as CSSProperties
                  }
                >
                  <button
                    type="button"
                    className="auth-sat"
                    data-active={active === index}
                    onPointerEnter={() => {
                      setHeld(true);
                      setActive(index);
                    }}
                    onPointerLeave={() => setHeld(false)}
                    onFocus={() => {
                      setHeld(true);
                      setActive(index);
                    }}
                    onBlur={() => setHeld(false)}
                  >
                    <span className="auth-sat-dot" />
                    <span className="auth-sat-copy">
                      <span className="auth-signal-title">{signal.title}</span>
                      <span className="auth-signal-body">{signal.body}</span>
                    </span>
                  </button>
                </div>
              ))}
            </div>

            <motion.p
              className="auth-folio-kicker"
              initial={reducedMotion ? false : { opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ ...instant, delay: reducedMotion ? 0 : 0.08 }}
            >
              {rail.kicker}
            </motion.p>
            <motion.h2
              className="auth-folio-headline"
              initial={reducedMotion ? false : { opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ ...instant, delay: reducedMotion ? 0 : 0.14 }}
            >
              {rail.headline}
            </motion.h2>
          </section>

          <motion.main
            className="auth-entry"
            initial={reducedMotion ? false : { opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ ...instant, delay: reducedMotion ? 0 : 0.18 }}
          >
            {kicker ? <p className="auth-kicker">{kicker}</p> : null}
            <h1 className="auth-title">{title}</h1>
            {lede ? <p className="auth-lede">{lede}</p> : null}
            {notice}
            {children}
            {footer ? <div className="auth-footer">{footer}</div> : null}
          </motion.main>
        </div>
      </div>
    </div>
  );
}

export function AuthField({
  id,
  label,
  hint,
  action,
  children,
}: {
  id: string;
  label: string;
  hint?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="auth-field">
      <div className="auth-field-head">
        <label htmlFor={id} className="auth-label">
          {label}
        </label>
        {action}
      </div>
      {children}
      {hint ? <div className="auth-hint">{hint}</div> : null}
    </div>
  );
}

export function AuthInput({ className, ...props }: ComponentProps<typeof Input>) {
  return <Input className={cn("auth-input", className)} {...props} />;
}

export function AuthSubmit({
  children,
  className,
  ...props
}: ComponentProps<typeof Button>) {
  return (
    <Button className={cn("auth-submit", className)} {...props}>
      {children}
    </Button>
  );
}

export function AuthNotice({
  tone = "neutral",
  children,
}: {
  tone?: "ok" | "invite" | "warn" | "neutral";
  children: ReactNode;
}) {
  return (
    <p className="auth-notice" data-tone={tone}>
      {children}
    </p>
  );
}

export function AuthTextLink({
  children,
  className,
  ...props
}: ComponentProps<typeof Button>) {
  return (
    <Button type="button" variant="link" className={cn("auth-text-link", className)} {...props}>
      {children}
    </Button>
  );
}
