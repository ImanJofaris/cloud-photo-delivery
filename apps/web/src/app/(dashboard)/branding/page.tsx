import { BrandingForm } from "@/features/branding/branding-form"

export default function BrandingPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Branding</h1>
        <p className="text-sm text-muted-foreground">
          This is what guests see on every gallery you deliver.
        </p>
      </div>
      <BrandingForm />
    </div>
  )
}
