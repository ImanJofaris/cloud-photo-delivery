"use client"

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import type { components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { deviceKeys } from "./keys"

type Device = components["schemas"]["Device"]
type DeviceWithKey = components["schemas"]["DeviceWithKey"]
type DeviceList = components["schemas"]["DeviceList"]
type CreateDeviceRequest = components["schemas"]["CreateDeviceRequest"]
type UpdateDeviceRequest = components["schemas"]["UpdateDeviceRequest"]

export function useDevices() {
  return useQuery({
    queryKey: deviceKeys.list(),
    queryFn: () => apiCall<DeviceList>((client) => client.GET("/devices")),
  })
}

export function useCreateDevice() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: CreateDeviceRequest) =>
      apiCall<DeviceWithKey>((client) =>
        client.POST("/devices", { body: input })
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deviceKeys.lists() })
    },
  })
}

export function useRenameDevice(deviceId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: UpdateDeviceRequest) =>
      apiCall<Device>((client) =>
        client.PATCH("/devices/{deviceID}", {
          params: { path: { deviceID: deviceId } },
          body: input,
        })
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deviceKeys.lists() })
    },
  })
}

export function useRotateDeviceKey(deviceId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () =>
      apiCall<DeviceWithKey>((client) =>
        client.POST("/devices/{deviceID}/rotate", {
          params: { path: { deviceID: deviceId } },
        })
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deviceKeys.lists() })
    },
  })
}

export function useRevokeDevice(deviceId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () =>
      apiCall<void>((client) =>
        client.DELETE("/devices/{deviceID}", {
          params: { path: { deviceID: deviceId } },
        })
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deviceKeys.lists() })
    },
  })
}
