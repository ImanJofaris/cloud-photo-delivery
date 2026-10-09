"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { Check, ChevronRight, Palette } from "lucide-react"
import Link from "next/link"
import * as React from "react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { toast } from "sonner"

import { ApiError, type components } from "@workspace/api-client"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@workspace/ui/components/alert-dialog"
import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { useSubscription } from "@/features/billing/api"
import { useObjectUrl } from "@/lib/hooks/use-object-url"

import { useBranding, useUpdateBranding } from "./api"
import { AssetUploadField } from "./asset-upload"
import { BrandingPreview } from "./branding-preview"
import {
  brandingSchema,
  brandingValuesFrom,
  toBrandingPatch,
  type BrandingAssetKind,
  type BrandingValues,
} from "./schema"

type Branding = components["schemas"]["Branding"]

type AssetChange =
  | { mode: "set"; file: File; storageKey: string }
  | { mode: "clear" }

const HEX_PATTERN = /^#[0-9a-f]{6}$/
const PRESET_COLORS = ["#c2477f", "#e0873f", "#2f8f6b", "#3a6ea8", "#5b4a78"]

function brandingErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "VALIDATION_ERROR") {
      return "Please check the form and try again."
    }
    return "Could not save branding. Please try again."
  }
  return "Something went wrong. Please try again."
}

function Field({
  id,
  label,
  error,
  children,
}: {
  id: string
  label: string
  error?: string
  children: React.ReactNode
}) {
  return (
    <div className="min-w-0 space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}

function ColorField({
  id,
  label,
  value,
  onChange,
  error,
  hint,
  presets,
  disabled,
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
  hint?: string
  presets?: readonly string[]
  disabled?: boolean
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="color"
          aria-label={`${label} picker`}
          value={HEX_PATTERN.test(value) ? value : "#000000"}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value.toLowerCase())}
          className="size-9 shrink-0 cursor-pointer rounded-xl border bg-transparent p-0.5 disabled:cursor-not-allowed disabled:opacity-50"
        />
        <Input
          id={id}
          placeholder="#1a2b3c"
          className="w-32 font-mono"
          aria-invalid={Boolean(error)}
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
        />
        {presets ? (
          <div
            role="group"
            aria-label="Color presets"
            className="flex items-center gap-1.5"
          >
            {presets.map((preset) => (
              <button
                key={preset}
                type="button"
                aria-label={`Use ${preset}`}
                aria-pressed={value.toLowerCase() === preset}
                disabled={disabled}
                onClick={() => onChange(preset)}
                className="size-6 rounded-full outline-none ring-1 ring-foreground/15 transition focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
                style={{ backgroundColor: preset }}
              />
            ))}
          </div>
        ) : null}
        {hint && (
          <p className="text-xs text-muted-foreground md:ml-auto">{hint}</p>
        )}
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}

export function BrandingForm() {
  const brandingQuery = useBranding()
  const subscriptionQuery = useSubscription()

  if (brandingQuery.isPending || subscriptionQuery.isPending) {
    return (
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className="flex flex-col gap-4">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-64 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
        <Skeleton className="h-96 w-full" />
      </div>
    )
  }

  if (brandingQuery.isError) {
    return (
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle>Could not load branding</CardTitle>
          <CardDescription>Refresh the page to try again.</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const canEdit = subscriptionQuery.data?.plan.limits.branding ?? true

  return <BrandingFormBody branding={brandingQuery.data} canEdit={canEdit} />
}

function PlanGateNote() {
  return (
    <Card size="sm">
      <CardContent className="flex items-start gap-3">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Palette className="size-4" aria-hidden="true" />
        </div>
        <div className="min-w-0 space-y-1.5">
          <p className="text-sm font-medium">
            Custom branding is a Starter feature
          </p>
          <p className="text-xs text-muted-foreground">
            Upgrade to add your business name, logo, and colors to every
            gallery. Your brand applies to existing galleries too.
          </p>
          <Button
            variant="link"
            size="xs"
            className="px-0"
            render={<Link href="/billing" />}
            nativeButton={false}
          >
            Compare plans
            <ChevronRight aria-hidden="true" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function BrandingFormBody({
  branding,
  canEdit,
}: {
  branding: Branding
  canEdit: boolean
}) {
  const updateBranding = useUpdateBranding()
  const form = useForm<BrandingValues>({
    resolver: zodResolver(brandingSchema),
    defaultValues: brandingValuesFrom(branding),
  })

  const [logoChange, setLogoChange] = React.useState<AssetChange | null>(null)
  const [profileChange, setProfileChange] =
    React.useState<AssetChange | null>(null)
  const [clearTarget, setClearTarget] = React.useState<BrandingAssetKind | null>(
    null
  )

  const logoFile = logoChange?.mode === "set" ? logoChange.file : null
  const profileFile = profileChange?.mode === "set" ? profileChange.file : null
  const stagedLogoUrl = useObjectUrl(logoFile)
  const stagedProfileUrl = useObjectUrl(profileFile)

  const logoUrl =
    logoChange?.mode === "clear"
      ? null
      : (stagedLogoUrl ?? branding.logoUrl ?? null)
  const profileImageUrl =
    profileChange?.mode === "clear"
      ? null
      : (stagedProfileUrl ?? branding.profileImageUrl ?? null)
  const hasLogo = Boolean(branding.logoUrl) || logoChange?.mode === "set"
  const hasProfileImage =
    Boolean(branding.profileImageUrl) || profileChange?.mode === "set"

  const watched = useWatch({ control: form.control })
  const values: BrandingValues = {
    businessName: watched.businessName ?? "",
    primaryColor: watched.primaryColor ?? "",
    secondaryColor: watched.secondaryColor ?? "",
    contactEmail: watched.contactEmail ?? "",
    contactPhone: watched.contactPhone ?? "",
    websiteUrl: watched.websiteUrl ?? "",
  }

  async function onSubmit(raw: BrandingValues) {
    const parsed = brandingSchema.safeParse(raw)
    if (!parsed.success) return

    const patch = toBrandingPatch({
      initial: branding,
      values: parsed.data,
      logoKey:
        logoChange === null
          ? null
          : logoChange.mode === "set"
            ? logoChange.storageKey
            : "",
      profileImageKey:
        profileChange === null
          ? null
          : profileChange.mode === "set"
            ? profileChange.storageKey
            : "",
    })

    if (Object.keys(patch).length === 0) {
      toast.info("No branding changes to save")
      return
    }

    try {
      const saved = await updateBranding.mutateAsync(patch)
      setLogoChange(null)
      setProfileChange(null)
      form.reset(brandingValuesFrom(saved))
      toast.success("Branding saved")
    } catch (error) {
      form.setError("root", { message: brandingErrorMessage(error) })
    }
  }

  function confirmClear() {
    if (clearTarget === "logo") setLogoChange({ mode: "clear" })
    if (clearTarget === "profileImage") setProfileChange({ mode: "clear" })
    setClearTarget(null)
  }

  function discard() {
    form.reset(brandingValuesFrom(branding))
    setLogoChange(null)
    setProfileChange(null)
    form.clearErrors()
  }

  const { errors, isDirty, isSubmitting } = form.formState
  const hasAssetChanges = logoChange !== null || profileChange !== null
  const hasChanges = isDirty || hasAssetChanges

  return (
    <>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]"
        noValidate
      >
        <div className="flex min-w-0 flex-col gap-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Assets</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-5 sm:grid-cols-2">
              <AssetUploadField
                kind="logo"
                label="Logo"
                description="PNG, JPEG, or WebP up to 5 MB."
                previewUrl={logoUrl}
                hasAsset={hasLogo}
                disabled={!canEdit}
                onUploaded={({ file, storageKey }) =>
                  setLogoChange({ mode: "set", file, storageKey })
                }
                onClear={() => setClearTarget("logo")}
              />
              <AssetUploadField
                kind="profileImage"
                label="Profile image"
                description="Shown behind the logo in the gallery header."
                previewUrl={profileImageUrl}
                hasAsset={hasProfileImage}
                disabled={!canEdit}
                onUploaded={({ file, storageKey }) =>
                  setProfileChange({ mode: "set", file, storageKey })
                }
                onClear={() => setClearTarget("profileImage")}
              />
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle>Identity</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-5 sm:grid-cols-2">
              <Field
                id="businessName"
                label="Business name"
                error={errors.businessName?.message}
              >
                <Input
                  id="businessName"
                  placeholder="Booth Co"
                  aria-invalid={Boolean(errors.businessName)}
                  disabled={!canEdit}
                  {...form.register("businessName")}
                />
              </Field>
              <Field
                id="contactEmail"
                label="Contact email"
                error={errors.contactEmail?.message}
              >
                <Input
                  id="contactEmail"
                  type="email"
                  placeholder="hello@booth.example"
                  aria-invalid={Boolean(errors.contactEmail)}
                  disabled={!canEdit}
                  {...form.register("contactEmail")}
                />
              </Field>
              <Field
                id="contactPhone"
                label="Contact phone"
                error={errors.contactPhone?.message}
              >
                <Input
                  id="contactPhone"
                  placeholder="+60123456789"
                  aria-invalid={Boolean(errors.contactPhone)}
                  disabled={!canEdit}
                  {...form.register("contactPhone")}
                />
              </Field>
              <Field
                id="websiteUrl"
                label="Website"
                error={errors.websiteUrl?.message}
              >
                <Input
                  id="websiteUrl"
                  placeholder="https://booth.example"
                  aria-invalid={Boolean(errors.websiteUrl)}
                  disabled={!canEdit}
                  {...form.register("websiteUrl")}
                />
              </Field>
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle>Color</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Controller
                control={form.control}
                name="primaryColor"
                render={({ field }) => (
                  <ColorField
                    id="primaryColor"
                    label="Primary color"
                    value={field.value}
                    onChange={field.onChange}
                    error={errors.primaryColor?.message}
                    presets={PRESET_COLORS}
                    hint="Used on the gallery header, buttons, and accents."
                    disabled={!canEdit}
                  />
                )}
              />
              <Controller
                control={form.control}
                name="secondaryColor"
                render={({ field }) => (
                  <ColorField
                    id="secondaryColor"
                    label="Secondary color"
                    value={field.value}
                    onChange={field.onChange}
                    error={errors.secondaryColor?.message}
                    hint="Used in the gallery cover gradient."
                    disabled={!canEdit}
                  />
                )}
              />
            </CardContent>
          </Card>

          {errors.root && (
            <Alert variant="destructive">
              <AlertDescription>{errors.root.message}</AlertDescription>
            </Alert>
          )}

          {canEdit && (
            <div className="sticky bottom-4 z-10 flex flex-wrap items-center justify-between gap-3 rounded-2xl border bg-card/95 p-3 shadow-sm backdrop-blur">
              <p className="text-xs text-muted-foreground">
                Applies to every public gallery for your account.
              </p>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  disabled={!hasChanges || isSubmitting}
                  onClick={discard}
                >
                  Discard
                </Button>
                <Button type="submit" disabled={isSubmitting}>
                  {!isSubmitting && <Check aria-hidden="true" />}
                  {isSubmitting ? "Saving..." : "Save changes"}
                </Button>
              </div>
            </div>
          )}
        </div>

        <div className="flex min-w-0 flex-col gap-4 lg:sticky lg:top-4">
          <BrandingPreview
            values={values}
            logoUrl={logoUrl}
            profileImageUrl={profileImageUrl}
          />
          {!canEdit && <PlanGateNote />}
        </div>
      </form>

      <AlertDialog
        open={clearTarget !== null}
        onOpenChange={(open) => !open && setClearTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Remove {clearTarget === "logo" ? "the logo" : "the profile image"}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              The gallery falls back to its default header. You still need to
              save to apply the change.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmClear}>
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
