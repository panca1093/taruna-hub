"use client";

import { BookOpen, CheckCheck, ClipboardList } from "lucide-react";
import { useReducedMotion } from "framer-motion";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { CountdownRing } from "@/components/countdown-ring";
import { useAuthT } from "@/lib/i18n/dict/auth";

export function AuthLayout({ children, variant = "login" }: {
  children: React.ReactNode;
  variant?: "login" | "portal" | "register" | "verify";
}) {
  const t = useAuthT();
  const reducedMotion = useReducedMotion();
  const staff = variant === "portal";
  const title = staff ? t.staffStoryTitle : variant === "register" ? t.registerStoryTitle : variant === "verify" ? t.verifyStoryTitle : t.storyTitle;
  const description = staff ? t.staffStoryDescription : variant === "register" ? t.registerStoryDescription : variant === "verify" ? t.verifyStoryDescription : t.storyDescription;
  const caption = staff ? t.staffRingTitle : variant === "register" ? t.registerRingTitle : variant === "verify" ? t.verifyRingTitle : t.ringTitle;
  const subtitle = staff ? t.staffRingSubtitle : variant === "register" ? t.registerRingSubtitle : variant === "verify" ? t.verifyRingSubtitle : t.ringSubtitle;
  const panel = (
    <aside className="auth-story" aria-labelledby="auth-story-title">
      <div className="auth-story-copy auth-enter"><h2 id="auth-story-title">{title}</h2><p>{description}</p></div>
      <div className="auth-visual-group">
        <div className="auth-illustration" aria-hidden="true">
          <div className="auth-orbit" /><div className="auth-orbit auth-orbit-inner" />
          <CountdownRing value={76} size={200} strokeWidth={7} animate={!reducedMotion} trackClassName="stroke-accent/10">
            <span className="auth-book"><BookOpen className="size-10" strokeWidth={1.5} /></span>
          </CountdownRing>
          <span className="auth-spark auth-spark-one">✦</span><span className="auth-spark auth-spark-two">✧</span>
        </div>
        <div className="auth-caption auth-enter"><strong>{caption}</strong><span>{subtitle}</span></div>
        <div className="auth-feature-grid">
          <div className="auth-feature auth-enter"><span><ClipboardList className="size-4" /></span><div><small>{staff ? t.staffPracticeTag : t.practiceTag}</small><strong>{staff ? t.staffPractice : t.practice}</strong></div></div>
          <div className="auth-feature auth-enter"><span><CheckCheck className="size-4" /></span><div><strong>{staff ? t.staffReview : t.review}</strong><small>{staff ? t.staffReviewSubtitle : t.reviewSubtitle}</small></div></div>
        </div>
      </div>
      <div className="auth-story-footer auth-enter"><span className="auth-small-ring" aria-hidden="true" /><p>{staff ? t.staffFootnote : t.storyFootnote}</p></div>
    </aside>
  );
  const form = <section className="auth-panel"><div className="auth-form-wrap auth-enter">{children}</div><p className="auth-footer">{t.authFooter}</p></section>;
  return (
    <div className="auth-page">
      <header className="auth-header"><Logo className="focus-visible:rounded-lg focus-visible:ring-2 focus-visible:ring-focus" /><div className="flex items-center gap-2"><LanguageToggle /><span className="h-4 w-px bg-border" aria-hidden="true" /><ThemeToggle /></div></header>
      <main className="auth-main">{staff ? <>{form}{panel}</> : <>{panel}{form}</>}</main>
    </div>
  );
}
