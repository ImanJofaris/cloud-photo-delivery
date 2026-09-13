export default function GalleryNotFound() {
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-3 px-6 text-center">
      <p className="text-sm font-medium text-muted-foreground">404</p>
      <h1 className="text-2xl font-semibold tracking-tight">
        Gallery not found
      </h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        This gallery does not exist, is private, or has expired. Check the link
        with the person who shared it.
      </p>
    </main>
  )
}
