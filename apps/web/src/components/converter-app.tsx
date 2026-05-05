"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CategoryId } from "@/types";
import {
  applyCatalogHints,
  getCategoryById,
  DEFAULT_CATEGORY,
} from "@/config/categories";
import { getCatalog, type CatalogFamily } from "@/lib/api";
import Header from "./header";
import CategoryNav from "./category-nav";
import ConversionCard from "./conversion-card";

export default function ConverterApp() {
  const [activeCategory, setActiveCategory] =
    useState<CategoryId>(DEFAULT_CATEGORY);
  const [catalogFamilies, setCatalogFamilies] = useState<CatalogFamily[] | null>(
    null,
  );

  useEffect(() => {
    let mounted = true;
    getCatalog()
      .then((families) => {
        if (mounted) setCatalogFamilies(families);
      })
      .catch(() => {
        if (mounted) setCatalogFamilies(null);
      });
    return () => {
      mounted = false;
    };
  }, []);

  const handleCategoryChange = useCallback((id: CategoryId) => {
    setActiveCategory(id);
  }, []);

  const category = useMemo(
    () => applyCatalogHints(getCategoryById(activeCategory), catalogFamilies),
    [activeCategory, catalogFamilies],
  );

  return (
    <>
      <Header
        toolbar={
          <CategoryNav
            activeCategory={activeCategory}
            onChange={handleCategoryChange}
          />
        }
      />

      <main className="flex flex-1 items-start justify-center px-5 pb-10 pt-8 sm:px-8 sm:pb-12 sm:pt-10">
        <section className="w-full max-w-280">
          <ConversionCard category={category} />
        </section>
      </main>
    </>
  );
}
