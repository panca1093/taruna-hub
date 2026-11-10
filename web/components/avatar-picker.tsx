"use client";

import { AVATAR_KEYS, GradientAvatar } from "./gradient-avatar";

/** Radio grid of the built-in avatars. The caller saves; this only reports the pick. */
export function AvatarPicker({
  value,
  onChange,
  groupLabel,
  optionLabel,
}: {
  value: string | null;
  onChange: (key: string) => void;
  groupLabel: string;
  optionLabel: (index: number) => string;
}) {
  return (
    <div role="radiogroup" aria-label={groupLabel} className="grid grid-cols-5 gap-2 sm:grid-cols-7 sm:gap-2.5">
      {AVATAR_KEYS.map((key, index) => (
        <label
          key={key}
          className="relative grid aspect-square cursor-pointer place-items-center rounded-tile border-2 border-transparent p-1.5 hover:border-border has-checked:border-accent has-checked:bg-accent-soft has-focus-visible:ring-2 has-focus-visible:ring-focus"
        >
          <input
            type="radio"
            name="avatar"
            value={key}
            checked={value === key}
            onChange={() => onChange(key)}
            aria-label={optionLabel(index + 1)}
            className="absolute inset-0 cursor-pointer opacity-0"
          />
          <GradientAvatar user={{ name: key, avatar: key }} label="" size="fill" />
        </label>
      ))}
    </div>
  );
}
