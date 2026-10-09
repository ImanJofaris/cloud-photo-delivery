"use client"

import { Tabs, TabsList, TabsTrigger } from "@workspace/ui/components/tabs"

import { ANALYTICS_PANEL_WINDOWS } from "./schema"

export function analyticsWindowTabLabel(days: number): string {
  return `${days} days`
}

export function WindowTabs({
  days,
  onDaysChange,
}: {
  days: number
  onDaysChange: (days: number) => void
}) {
  return (
    <Tabs
      value={String(days)}
      onValueChange={(value) => onDaysChange(Number(value))}
    >
      <TabsList aria-label="Analytics window">
        {ANALYTICS_PANEL_WINDOWS.map((option) => (
          <TabsTrigger
            key={option}
            value={String(option)}
            className="px-3 text-xs font-semibold"
          >
            {analyticsWindowTabLabel(option)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}
