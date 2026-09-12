import { Geist, Geist_Mono } from "next/font/google"

import "@workspace/ui/globals.css"
import { Toaster } from "@workspace/ui/components/sonner"
import { cn } from "@workspace/ui/lib/utils"

import { ThemeProvider } from "@/components/theme-provider"
import { SessionProvider } from "@/lib/auth/session-provider"
import { QueryProvider } from "@/lib/query-provider"

const geist = Geist({ subsets: ["latin"], variable: "--font-sans" })

const fontMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
})

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn(
        "antialiased",
        fontMono.variable,
        "font-sans",
        geist.variable
      )}
    >
      <body>
        <ThemeProvider>
          <QueryProvider>
            <SessionProvider>
              {children}
              <Toaster />
            </SessionProvider>
          </QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
