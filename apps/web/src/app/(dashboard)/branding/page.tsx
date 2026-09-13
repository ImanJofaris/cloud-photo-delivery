import { BrandingForm } from "@/features/branding/branding-form"

export default function BrandingPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Branding</h1>
        <p className="text-sm text-muted-foreground">
          Logo, colors, and contact details for your public galleries.
        </p>
      </div>
      <BrandingForm />
    </div>
  )
}
