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

import { formatCount } from "@/features/analytics/format"
import { formatDate } from "@/features/events/format"

import { formatBytes } from "./format"

type AdminUser = components["schemas"]["AdminUser"]

export function AdminUsersTable({
  users,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: {
  users: AdminUser[]
  hasNextPage: boolean
  isFetchingNextPage: boolean
  onLoadMore: () => void
}) {
  return (
    <Card id="admin-users">
      <CardHeader>
        <CardTitle>Operators</CardTitle>
        <CardDescription>Every account on the platform.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {users.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            No operators yet.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">Business</TableHead>
                <TableHead scope="col">Email</TableHead>
                <TableHead scope="col">Role</TableHead>
                <TableHead scope="col">Events</TableHead>
                <TableHead scope="col">Storage</TableHead>
                <TableHead scope="col">Joined</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">
                    {user.businessName || "—"}
                  </TableCell>
                  <TableCell>{user.email}</TableCell>
                  <TableCell>
                    {user.isAdmin ? (
                      <Badge variant="secondary">Admin</Badge>
                    ) : (
                      <span className="text-muted-foreground">Operator</span>
                    )}
                  </TableCell>
                  <TableCell>{formatCount(user.eventCount)}</TableCell>
                  <TableCell>{formatBytes(user.storageBytes)}</TableCell>
                  <TableCell>{formatDate(user.createdAt)}</TableCell>
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
