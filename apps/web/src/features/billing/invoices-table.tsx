"use client"

import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table"

import { formatDate } from "@/features/events/format"

import { formatMYR } from "./format"

type Invoice = components["schemas"]["Invoice"]
type InvoiceStatus = components["schemas"]["InvoiceStatus"]

const STATUS_VARIANTS: Record<
  InvoiceStatus,
  "default" | "secondary" | "outline"
> = {
  open: "outline",
  paid: "secondary",
  void: "outline",
}

export function InvoicesTable({
  invoices,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: {
  invoices: Invoice[]
  hasNextPage: boolean
  isFetchingNextPage: boolean
  onLoadMore: () => void
}) {
  return (
    <Card id="invoices">
      <CardHeader>
        <CardTitle>Invoices</CardTitle>
        <CardDescription>Recent billing history.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {invoices.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            No invoices yet.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Issued</TableHead>
                <TableHead>Paid</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invoices.map((invoice) => (
                <TableRow key={invoice.id}>
                  <TableCell className="font-medium">
                    {formatMYR(invoice.amountCents, invoice.currency)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={STATUS_VARIANTS[invoice.status]}>
                      {invoice.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{formatDate(invoice.issuedAt)}</TableCell>
                  <TableCell>
                    {invoice.paidAt ? formatDate(invoice.paidAt) : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {hasNextPage && (
          <div className="flex justify-center">
            <Button
              variant="outline"
              disabled={isFetchingNextPage}
              onClick={onLoadMore}
            >
              {isFetchingNextPage ? "Loading..." : "Load more"}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
