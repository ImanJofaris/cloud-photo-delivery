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
import { cn } from "@workspace/ui/lib/utils"

import { formatDate } from "@/features/events/format"

import { formatInvoiceRef, formatMYR } from "./format"

type Invoice = components["schemas"]["Invoice"]
type InvoiceStatus = components["schemas"]["InvoiceStatus"]

const STATUS_LABELS: Record<InvoiceStatus, string> = {
  open: "Open",
  paid: "Paid",
  void: "Void",
}

const STATUS_STYLES: Record<InvoiceStatus, string> = {
  open: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  paid: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  void: "bg-muted text-muted-foreground",
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
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>Invoices</CardTitle>
          <CardDescription>Newest first</CardDescription>
        </div>
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
                <TableHead>Invoice</TableHead>
                <TableHead>Date</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invoices.map((invoice) => (
                <TableRow key={invoice.id}>
                  <TableCell className="font-mono text-xs" title={invoice.id}>
                    {formatInvoiceRef(invoice.id)}
                  </TableCell>
                  <TableCell>{formatDate(invoice.issuedAt)}</TableCell>
                  <TableCell className="font-medium">
                    {formatMYR(invoice.amountCents, invoice.currency)}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      className={cn(STATUS_STYLES[invoice.status])}
                    >
                      {STATUS_LABELS[invoice.status]}
                    </Badge>
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
              {isFetchingNextPage ? "Loading..." : "Load older invoices"}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
