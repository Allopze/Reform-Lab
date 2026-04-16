"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  getAdminUsers,
  updateUserRole,
  type AdminUser,
  type AdminUserFilter,
  type AdminUserPage,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const ROLE_BADGE: Record<string, string> = {
  admin: "border-violet-200 bg-violet-50 text-violet-700",
  user: "border-stone-200 bg-stone-100 text-stone-600",
};
const PAGE_SIZE = 30;

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat("es", {
      day: "2-digit",
      month: "short",
      year: "numeric",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

export default function AdminUsersTable() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const t = useTranslations("adminUsers");

  const [page, setPage] = useState<AdminUserPage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [updating, setUpdating] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<"" | "admin" | "user">("");
  const [offset, setOffset] = useState(0);

  const fetchUsers = useCallback(async () => {
    try {
      setError(null);
      const filter: AdminUserFilter = {
        limit: PAGE_SIZE,
        offset,
      };
      if (search.trim()) filter.q = search.trim();
      if (roleFilter) filter.role = roleFilter;
      const data = await getAdminUsers(filter);
      setPage(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("loadError"));
    }
  }, [offset, roleFilter, search, t]);

  useEffect(() => {
    if (loading) return;
    if (!user || user.role !== "admin") {
      router.replace("/usuario");
      return;
    }
    fetchUsers();
  }, [user, loading, router, fetchUsers]);

  async function handleToggleRole(target: AdminUser) {
    const newRole = target.role === "admin" ? "user" : "admin";
    setUpdating(target.id);
    try {
      await updateUserRole(target.id, newRole);
      await fetchUsers();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("loadError"));
    } finally {
      setUpdating(null);
    }
  }

  if (loading || (!page && !error)) {
    return <p className="py-8 text-center text-sm text-stone-400">{t("loading")}</p>;
  }
  if (error) {
    return <p className="py-8 text-center text-sm text-rose-600">{error}</p>;
  }

  if (!page) return null;

  const users = page.users;
  const totalPages = Math.ceil(page.total / PAGE_SIZE);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  return (
    <div className="mt-6 space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <select
          value={roleFilter}
          onChange={(e) => {
            setRoleFilter(e.target.value as "" | "admin" | "user");
            setOffset(0);
          }}
          className="h-9 rounded-lg border border-stone-200 bg-white px-3 text-sm text-stone-700"
        >
          <option value="">{t("allRoles")}</option>
          <option value="admin">{t("roleAdmin")}</option>
          <option value="user">{t("roleUser")}</option>
        </select>

        <input
          type="text"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setOffset(0);
          }}
          placeholder={t("searchPlaceholder")}
          className="h-9 w-64 rounded-lg border border-stone-200 bg-white px-3 text-sm text-stone-700 placeholder:text-stone-400"
        />

        <span className="ml-auto text-xs text-stone-500">
          {t("totalUsers", { count: page.total })}
        </span>
      </div>

      <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
      <div className="border-b border-stone-200 px-5 py-4">
        <p className="text-sm text-stone-500">{t("description")}</p>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-stone-200 bg-stone-50 text-xs font-medium uppercase tracking-wider text-stone-500">
            <tr>
              <th className="px-5 py-3">{t("headerName")}</th>
              <th className="px-5 py-3">{t("headerEmail")}</th>
              <th className="px-5 py-3">{t("headerTeam")}</th>
              <th className="px-5 py-3">{t("headerRole")}</th>
              <th className="px-5 py-3">{t("headerCreatedAt")}</th>
              <th className="px-5 py-3">{t("headerActions")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-stone-100">
            {users.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-5 py-8 text-center text-stone-400">
                  {t("emptyUsers")}
                </td>
              </tr>
            ) : (
              users.map((u) => {
                const isSelf = user?.id === u.id;
                return (
                  <tr key={u.id} className="hover:bg-stone-50/60">
                    <td className="whitespace-nowrap px-5 py-3 font-medium text-stone-900">
                      {u.name}
                    </td>
                    <td className="px-5 py-3 text-stone-600">{u.email}</td>
                    <td className="px-5 py-3 text-stone-500">{u.team || "—"}</td>
                    <td className="px-5 py-3">
                      <span
                        className={`inline-block rounded-full border px-2.5 py-0.5 text-xs font-medium ${ROLE_BADGE[u.role] || ROLE_BADGE.user}`}
                      >
                        {u.role}
                      </span>
                    </td>
                    <td className="px-5 py-3 text-stone-500">
                      {formatDate(u.createdAt)}
                    </td>
                    <td className="px-5 py-3">
                      {isSelf ? (
                        <span className="text-xs text-stone-400">{t("you")}</span>
                      ) : (
                        <button
                          onClick={() => handleToggleRole(u)}
                          disabled={updating === u.id}
                          className="rounded-md border border-stone-200 px-3 py-1 text-xs font-medium text-stone-700 hover:bg-stone-50 disabled:opacity-50"
                        >
                          {updating === u.id
                            ? "..."
                            : u.role === "admin"
                              ? t("demote")
                              : t("promote")}
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between border-t border-stone-200 px-5 py-3 text-sm text-stone-600">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            className="rounded-lg border border-stone-200 px-3 py-1.5 text-sm transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t("prev")}
          </button>
          <span>{t("pageOf", { current: currentPage, total: totalPages })}</span>
          <button
            type="button"
            disabled={currentPage >= totalPages}
            onClick={() => setOffset(offset + PAGE_SIZE)}
            className="rounded-lg border border-stone-200 px-3 py-1.5 text-sm transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t("next")}
          </button>
        </div>
      )}
      </div>
    </div>
  );
}
