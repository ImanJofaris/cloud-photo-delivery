"use client"

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select"

import { ANALYTICS_WINDOWS } from "./schema"

export function analyticsWindowLabel(days: number): string {
  return `Last ${days} days`
}

export function WindowSelect({
  days,
  onDaysChange,
}: {
  days: number
  onDaysChange: (days: number) => void
}) {
  return (
    <Select
      value={String(days)}
      onValueChange={(value) => onDaysChange(Number(value))}
    >
      <SelectTrigger className="w-40" aria-label="Analytics window">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {ANALYTICS_WINDOWS.map((option) => (
          <SelectItem key={option} value={String(option)}>
            {analyticsWindowLabel(option)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
