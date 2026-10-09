export type DeviceSummaryInput = {
  name: string
  assignedEventId: string | null
  revokedAt: string | null
  lastUsedAt: string | null
}

export type DeviceSummary = {
  activeCount: number
  scopedEventCount: number
  revokedCount: number
  lastUpload: { deviceName: string; lastUsedAt: string } | null
}

export function summarizeDevices(devices: DeviceSummaryInput[]): DeviceSummary {
  const active = devices.filter((device) => !device.revokedAt)
  const scopedEventIds = new Set(
    active
      .map((device) => device.assignedEventId)
      .filter((eventId): eventId is string => eventId !== null)
  )

  let lastUpload: DeviceSummary["lastUpload"] = null
  let lastUploadAt = Number.NEGATIVE_INFINITY
  for (const device of active) {
    if (!device.lastUsedAt) continue
    const at = Date.parse(device.lastUsedAt)
    if (Number.isNaN(at) || at <= lastUploadAt) continue
    lastUploadAt = at
    lastUpload = { deviceName: device.name, lastUsedAt: device.lastUsedAt }
  }

  return {
    activeCount: active.length,
    scopedEventCount: scopedEventIds.size,
    revokedCount: devices.length - active.length,
    lastUpload,
  }
}
