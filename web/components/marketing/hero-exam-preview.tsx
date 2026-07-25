"use client";

import { motion } from "framer-motion";
import { Card, Chip } from "@heroui/react";
import { CircleCheck, Timer, Trophy } from "lucide-react";
import { CountdownRing } from "../countdown-ring";
import { useLandingT } from "@/lib/i18n/dict/landing";

export function HeroExamPreview() {
  const t = useLandingT();
  return (
    <div className="relative mx-auto w-full max-w-sm">
      <motion.div
        initial={{ opacity: 0, y: 16, rotate: -3 }}
        animate={{ opacity: 1, y: 0, rotate: -3 }}
        transition={{ duration: 0.6, delay: 0.15, ease: [0.32, 0.72, 0, 1] }}
        className="absolute -bottom-6 -left-6 w-52 rounded-xl border border-border bg-surface p-4 shadow-overlay"
      >
        <div className="flex items-center gap-2">
          <span className="inline-flex size-7 items-center justify-center rounded-full bg-success-soft text-success-soft-foreground">
            <Trophy className="size-3.5" />
          </span>
          <p className="text-xs font-medium text-muted">{t.heroPreviewResultLabel}</p>
        </div>
        <p className="mt-2 font-display text-2xl font-semibold text-foreground">
          91<span className="text-sm font-normal text-muted">/100</span>
        </p>
        <p className="text-xs text-muted">{t.heroPreviewRank(1, 22)}</p>
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 24, rotate: 2 }}
        animate={{ opacity: 1, y: 0, rotate: 2 }}
        transition={{ duration: 0.6, ease: [0.32, 0.72, 0, 1] }}
      >
        <Card className="relative w-full gap-4 border-border/80 p-5 shadow-overlay">
          <div className="flex items-center justify-between">
            <Chip size="sm" className="bg-accent-soft text-accent-soft-foreground font-medium">
              Matematika · {t.heroPreviewQuestionProgress(4, 5)}
            </Chip>
            <CountdownRing value={62} size={44} strokeWidth={5}>
              <Timer className="size-4 text-accent" />
            </CountdownRing>
          </div>
          <div className="space-y-2.5">
            <p className="text-sm leading-relaxed text-foreground">
              Akar-akar persamaan <span className="font-mono">x² − 5x + 6 = 0</span> adalah&hellip;
            </p>
            <div className="space-y-1.5">
              {["2 dan 3", "1 dan 6", "-2 dan -3"].map((opt, i) => (
                <div
                  key={opt}
                  className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-xs ${
                    i === 0 ? "border-accent bg-accent-soft text-accent-soft-foreground" : "border-border text-muted"
                  }`}
                >
                  <span
                    className={`flex size-4 items-center justify-center rounded-full border text-[9px] ${
                      i === 0 ? "border-accent bg-accent text-white" : "border-border"
                    }`}
                  >
                    {i === 0 && <CircleCheck className="size-3" />}
                  </span>
                  {opt}
                </div>
              ))}
            </div>
          </div>
        </Card>
      </motion.div>
    </div>
  );
}
