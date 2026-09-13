import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

vi.mock("./api", () => ({
  useEventPublicUrl: vi.fn(),
}))

vi.mock("@/lib/auth/api", () => ({
  apiBlobCall: vi.fn(),
}))

import { apiBlobCall } from "@/lib/auth/api"

import { useEventPublicUrl } from "./api"
import { buildPrintDocument, QrDialog } from "./qr-dialog"

const createObjectURL = vi.fn(() => "blob:mock")
const revokeObjectURL = vi.fn()

function renderDialog() {
  return render(
    <QrDialog
      eventId="event-1"
      eventName="Sarah & John's Wedding"
      open
      onOpenChange={vi.fn()}
    />
  )
}

describe("QrDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    URL.createObjectURL = createObjectURL
    URL.revokeObjectURL = revokeObjectURL
    vi.mocked(useEventPublicUrl).mockReturnValue({
      data: { url: "https://public.test/e/sarah-john" },
    } as unknown as ReturnType<typeof useEventPublicUrl>)
    vi.mocked(apiBlobCall)
      .mockResolvedValueOnce(new Blob(["png"], { type: "image/png" }))
      .mockResolvedValueOnce(
        new Blob(["<svg><rect /></svg>"], { type: "image/svg+xml" })
      )
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("renders the PNG preview and revokes object URLs on unmount", async () => {
    const { unmount } = renderDialog()

    const image = await screen.findByRole("img", {
      name: "QR code for Sarah & John's Wedding",
    })
    expect(image).toHaveAttribute("src", "blob:mock")
    expect(createObjectURL).toHaveBeenCalledTimes(2)

    unmount()

    await waitFor(() =>
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock")
    )
  })

  it("downloads the PNG and SVG with distinct filenames", async () => {
    const clicked: HTMLAnchorElement[] = []
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(
      function (this: HTMLAnchorElement) {
        clicked.push(this)
      }
    )

    renderDialog()
    await screen.findByRole("img", {
      name: "QR code for Sarah & John's Wedding",
    })

    await userEvent.click(screen.getByRole("button", { name: /Download PNG/ }))
    await userEvent.click(screen.getByRole("button", { name: /Download SVG/ }))

    expect(clicked.map((anchor) => anchor.download)).toEqual([
      "sarah-john-s-wedding-qr.png",
      "sarah-john-s-wedding-qr.svg",
    ])
    expect(clicked[0].href).toContain("blob:mock")
  })

  it("shows an error when the blobs cannot be loaded", async () => {
    vi.mocked(apiBlobCall).mockReset()
    vi.mocked(apiBlobCall).mockRejectedValue(new Error("nope"))

    renderDialog()

    expect(
      await screen.findByText(/Could not load the QR code/)
    ).toBeInTheDocument()
  })
})

describe("buildPrintDocument", () => {
  it("embeds the inline SVG, event name, and print instruction", () => {
    const html = buildPrintDocument({
      svg: "<svg><rect /></svg>",
      eventName: "Sarah & John's <Wedding>",
    })

    expect(html).toContain("<svg><rect /></svg>")
    expect(html).toContain("Sarah &amp; John&#39;s &lt;Wedding&gt;")
    expect(html).toContain("Scan to view photos")
    expect(html).toContain("window.print()")
  })
})
