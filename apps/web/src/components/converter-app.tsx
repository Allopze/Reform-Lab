"use client";

import { useState, useCallback } from "react";
import type { CategoryId } from "@/types";
import { getCategoryById, DEFAULT_CATEGORY } from "@/config/categories";
import Header from "./header";
import CategoryNav from "./category-nav";
import ConversionCard from "./conversion-card";

export default function ConverterApp() {
  const [activeCategory, setActiveCategory] =
    useState<CategoryId>(DEFAULT_CATEGORY);

  const handleCategoryChange = useCallback((id: CategoryId) => {
    setActiveCategory(id);
  }, []);

  const category = getCategoryById(activeCategory);

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

      <main className="flex flex-1 items-start justify-center px-5 pb-10 pt-5 sm:px-8 sm:pb-12 sm:pt-7">
        <section className="w-full max-w-280">
          <ConversionCard key={category.id} category={category} />
        </section>
      </main>
    </>
  );
}
