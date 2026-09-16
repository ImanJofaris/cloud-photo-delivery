"use client"

import { zodResolver } from "@hookform/resolvers/zod"
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

function brandingErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "VALIDATION_ERROR") {
      return "Please check the form and try again."
    }
    return "Could not save branding. Please try again."
  }
  return "Something went wrong. Please try again."
}

function ColorField({
  id,
  label,
  value,
  onChange,
  error,
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex items-center gap-2">
        <input
          type="color"
          aria-label={`${label} picker`}
          value={HEX_PATTERN.test(value) ? value : "#000000"}
          onChange={(event) => onChange(event.target.value.toLowerCase())}
          className="size-8 shrink-0 cursor-pointer rounded-md border bg-transparent p-0.5"
        />
        <Input
          id={id}
          placeholder="#1a2b3c"
          aria-invalid={Boolean(error)}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        />
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
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-96 w-full" />
      </div>
    )
  }

  if (brandingQuery.isError) {
    return (
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle>Could not load branding</CardTitle>
          <CardDescription>
            Refresh the page to try again.
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const plan = subscriptionQuery.data?.plan
  if (plan && !plan.limits.branding) {
    return (
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle>Custom branding is a Starter feature</CardTitle>
          <CardDescription>
            Upgrade to add your business name, logo, and colors to every
            gallery.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button render={<Link href="/billing" />} nativeButton={false}>
            View plans
          </Button>
        </CardContent>
      </Card>
    )
  }

  return <BrandingFormBody branding={brandingQuery.data} />
}

function BrandingFormBody({ branding }: { branding: Branding }) {
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
  const [saveFailed, setSaveFailed] = React.useState(false)

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
      setSaveFailed(false)
      form.reset(brandingValuesFrom(saved))
      toast.success("Branding saved")
    } catch (error) {
      setSaveFailed(true)
      form.setError("root", { message: brandingErrorMessage(error) })
    }
  }

  function confirmClear() {
    if (clearTarget === "logo") setLogoChange({ mode: "clear" })
    if (clearTarget === "profileImage")
      setProfileChange({ mode: "clear" })
    setClearTarget(null)
  }

  const { errors, isSubmitting } = form.formState
  const submitLabel = saveFailed ? "Save again" : "Save branding"

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
      <Card>
        <CardHeader>
          <CardTitle>Branding</CardTitle>
          <CardDescription>
            Applied to every public gallery for your account.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-5"
            noValidate
          >
            <div className="space-y-2">
              <Label htmlFor="businessName">Business name</Label>
              <Input
                id="businessName"
                placeholder="Booth Co"
                aria-invalid={Boolean(errors.businessName)}
                {...form.register("businessName")}
              />
              {errors.businessName && (
                <p className="text-sm text-destructive">
                  {errors.businessName.message}
                </p>
              )}
            </div>

            <div className="grid gap-5 sm:grid-cols-2">
              <AssetUploadField
                kind="logo"
                label="Logo"
                description="PNG, JPEG, or WebP up to 5 MB."
                previewUrl={logoUrl}
                hasAsset={hasLogo}
                onUploaded={({ file, storageKey }) =>
                  setLogoChange({ mode: "set", file, storageKey })
                }
                onClear={() => setClearTarget("logo")}
              />
              <AssetUploadField
                kind="profileImage"
                label="Profile image"
                description="Used as the gallery banner. Up to 5 MB."
                previewUrl={profileImageUrl}
                hasAsset={hasProfileImage}
                onUploaded={({ file, storageKey }) =>
                  setProfileChange({ mode: "set", file, storageKey })
                }
                onClear={() => setClearTarget("profileImage")}
              />
            </div>

            <div className="grid gap-5 sm:grid-cols-2">
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
                  />
                )}
              />
            </div>

            <div className="grid gap-5 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="contactEmail">Contact email</Label>
                <Input
                  id="contactEmail"
                  type="email"
                  placeholder="hello@booth.example"
                  aria-invalid={Boolean(errors.contactEmail)}
                  {...form.register("contactEmail")}
                />
                {errors.contactEmail && (
                  <p className="text-sm text-destructive">
                    {errors.contactEmail.message}
                  </p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="contactPhone">Contact phone</Label>
                <Input
                  id="contactPhone"
                  placeholder="+60123456789"
                  aria-invalid={Boolean(errors.contactPhone)}
                  {...form.register("contactPhone")}
                />
                {errors.contactPhone && (
                  <p className="text-sm text-destructive">
                    {errors.contactPhone.message}
                  </p>
                )}
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="websiteUrl">Website</Label>
              <Input
                id="websiteUrl"
                placeholder="https://booth.example"
                aria-invalid={Boolean(errors.websiteUrl)}
                {...form.register("websiteUrl")}
              />
              {errors.websiteUrl && (
                <p className="text-sm text-destructive">
                  {errors.websiteUrl.message}
                </p>
              )}
            </div>

            {errors.root && (
              <Alert variant="destructive">
                <AlertDescription>{errors.root.message}</AlertDescription>
              </Alert>
            )}

            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Saving..." : submitLabel}
            </Button>
          </form>
        </CardContent>
      </Card>

      <BrandingPreview
        values={values}
        logoUrl={logoUrl}
        profileImageUrl={profileImageUrl}
      />

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
    </div>
  )
}
