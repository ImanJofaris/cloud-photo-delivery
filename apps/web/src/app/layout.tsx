import { IBM_Plex_Mono, Plus_Jakarta_Sans } from "next/font/google"

import "@workspace/ui/globals.css"
import { Toaster } from "@workspace/ui/components/sonner"
import { cn } from "@workspace/ui/lib/utils"

import { ThemeProvider } from "@/components/theme-provider"
import { SessionProvider } from "@/lib/auth/session-provider"
import { QueryProvider } from "@/lib/query-provider"

const plusJakartaSans = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-sans",
})

const ibmPlexMono = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
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
        ibmPlexMono.variable,
        "font-sans",
        plusJakartaSans.variable
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
