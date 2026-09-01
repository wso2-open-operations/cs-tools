// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type UIEvent,
} from "react";
import { useGetProducts } from "@features/project-details/api/useGetProducts";
import { useSearchProductVersions } from "@features/project-details/api/useSearchProductVersions";
import { usePostDeploymentProduct } from "@features/project-details/api/usePostDeploymentProduct";
import type {
  ProductItem,
  ProductVersionItem,
} from "@features/project-details/types/products";
import {
  ADD_PRODUCT_MODAL_INITIAL_FORM,
  parseValidNumber,
  type AddProductModalFormState,
} from "@features/project-details/utils/addProductModal";

export type AddedProductSummary = {
  productId: string;
  productLabel: string;
  versionId: string;
  versionLabel: string;
};

export type UseProductFormResult = {
  form: AddProductModalFormState;
  products: ProductItem[];
  versions: ProductVersionItem[];
  isLoadingProducts: boolean;
  isFetchingProducts: boolean;
  isLoadingVersions: boolean;
  isFetchingVersions: boolean;
  canLoadMoreProducts: boolean;
  canLoadMoreVersions: boolean;
  isFetchingMoreProducts: boolean;
  isFetchingMoreVersions: boolean;
  handleProductsScroll: (event: UIEvent<HTMLElement>) => void;
  handleVersionsScroll: (event: UIEvent<HTMLElement>) => void;
  handleProductChange: (event: ChangeEvent<HTMLInputElement>) => void;
  handleVersionChange: (event: ChangeEvent<HTMLInputElement>) => void;
  handleTextChange: (
    field: "cores" | "tps" | "description",
  ) => (event: ChangeEvent<HTMLInputElement>) => void;
  isValid: boolean;
  isSubmitting: boolean;
  /**
   * Submits the current form via usePostDeploymentProduct. Resolves with a
   * summary of the product/version that was just added (looked up from the
   * currently loaded pages before the form is reset) so a caller can render
   * a "products added so far" list. Rejects with the underlying error - the
   * caller decides how to surface it (close-and-report vs. keep dialog open).
   */
  submit: () => Promise<AddedProductSummary>;
  /** Clears the form (and product/version pagination state) for a fresh entry. */
  resetForm: () => void;
};

/**
 * Encapsulates the paginated product/version search + product-add mutation
 * shared by the standalone AddProductModal and the add-deployment wizard's
 * product step. Pure form/data logic - no dialog chrome or close/open
 * decisions, those stay with the caller.
 *
 * @param {string} deploymentId - Deployment the product is being added to.
 * @param {string} projectId - Owning project id, sent in the request body.
 * @returns {UseProductFormResult} Form state, paginated options, and submit/reset handlers.
 */
export function useProductForm(
  deploymentId: string,
  projectId: string,
): UseProductFormResult {
  const [form, setForm] = useState<AddProductModalFormState>(() => ({
    ...ADD_PRODUCT_MODAL_INITIAL_FORM,
  }));
  const [productOffset, setProductOffset] = useState(0);
  const [versionOffset, setVersionOffset] = useState(0);
  const [products, setProducts] = useState<ProductItem[]>([]);
  const [versions, setVersions] = useState<ProductVersionItem[]>([]);
  const previousProductIdRef = useRef<string>("");
  const [cachedProductsTotalRecords, setCachedProductsTotalRecords] = useState<
    number | null
  >(null);
  const [cachedVersionsTotalRecords, setCachedVersionsTotalRecords] = useState<
    number | null
  >(null);

  const [productsLoadMorePending, setProductsLoadMorePending] = useState(false);
  const [versionsLoadMorePending, setVersionsLoadMorePending] = useState(false);
  const accumulatedProductsLengthRef = useRef(0);
  const accumulatedVersionsLengthRef = useRef(0);

  const productsRef = useRef<ProductItem[]>([]);
  const versionsRef = useRef<ProductVersionItem[]>([]);

  useLayoutEffect(() => {
    productsRef.current = products;
    accumulatedProductsLengthRef.current = products.length;
  }, [products]);

  useLayoutEffect(() => {
    versionsRef.current = versions;
    accumulatedVersionsLengthRef.current = versions.length;
  }, [versions]);

  // Only clears form values and version state (versions genuinely depend on
  // which product is selected, so they're stale once the form resets). The
  // product catalog itself (products/productOffset/its cached total) is NOT
  // cleared here: it's independent of any particular submission, and since
  // useGetProducts is keyed on [offset, limit] alone, resetting productOffset
  // back to the value it already was would never trigger a refetch - leaving
  // `products` stuck at [] with no way to repopulate it (the bug this fixes:
  // "Add another product" after a successful add showed an empty product
  // list, because the catalog got wiped without a matching refetch).
  const resetForm = useCallback(() => {
    setForm({ ...ADD_PRODUCT_MODAL_INITIAL_FORM });
    setVersionOffset(0);
    setVersions([]);
    previousProductIdRef.current = "";
    setVersionsLoadMorePending(false);
    setCachedVersionsTotalRecords(null);
  }, []);

  const {
    data: productsPage,
    isLoading: isLoadingProducts,
    isFetching: isFetchingProducts,
  } = useGetProducts({
    offset: productOffset,
    limit: 10,
  });

  /* eslint-disable react-hooks/set-state-in-effect -- paginated Select: load-more flags + merged list rows */
  useEffect(() => {
    if (!isFetchingProducts) {
      setProductsLoadMorePending(false);
    }
  }, [isFetchingProducts]);

  const {
    data: versionsPage,
    isLoading: isLoadingVersions,
    isFetching: isFetchingVersions,
  } = useSearchProductVersions(form.productId, {
    limit: 10,
    offset: versionOffset,
  });

  useEffect(() => {
    if (!isFetchingVersions) {
      setVersionsLoadMorePending(false);
    }
  }, [isFetchingVersions]);

  useEffect(() => {
    if (!productsPage) return;

    const pageItems = productsPage.products ?? [];
    const offset = productsPage.offset ?? 0;
    const pageLimit =
      typeof productsPage.limit === "number" && productsPage.limit > 0
        ? productsPage.limit
        : 10;

    const applyServerProductsTotal = (): void => {
      if (
        typeof productsPage.totalRecords === "number" &&
        !Number.isNaN(productsPage.totalRecords)
      ) {
        setCachedProductsTotalRecords(productsPage.totalRecords);
      }
    };

    if (offset > 0 && pageItems.length === 0) {
      setCachedProductsTotalRecords(accumulatedProductsLengthRef.current);
      return;
    }

    if (offset === 0) {
      setProducts(pageItems);
      if (pageItems.length < pageLimit) {
        setCachedProductsTotalRecords(pageItems.length);
      } else {
        applyServerProductsTotal();
      }
      return;
    }

    const prevProducts = productsRef.current;
    const prevProductIds = new Set(
      prevProducts.map((p) => p.id).filter((id): id is string => Boolean(id)),
    );
    const newProductItems = pageItems.filter(
      (p) =>
        typeof p.id === "string" &&
        p.id.length > 0 &&
        !prevProductIds.has(p.id),
    );
    const mergedProductsLen = prevProducts.length + newProductItems.length;

    if (newProductItems.length === 0) {
      setCachedProductsTotalRecords(prevProducts.length);
      return;
    }

    if (pageItems.length < pageLimit) {
      setCachedProductsTotalRecords(mergedProductsLen);
    } else {
      applyServerProductsTotal();
    }
    setProducts([...prevProducts, ...newProductItems]);
  }, [productsPage]);

  const productsTotalRecords = Number.isFinite(cachedProductsTotalRecords)
    ? (cachedProductsTotalRecords as number)
    : Number.isFinite(productsPage?.totalRecords)
      ? (productsPage!.totalRecords as number)
      : products.length;
  const canLoadMoreProducts = products.length < productsTotalRecords;

  const handleProductsScroll = useCallback(
    (event: UIEvent<HTMLElement>) => {
      const target = event.currentTarget;
      if (
        productsLoadMorePending ||
        !canLoadMoreProducts ||
        isLoadingProducts ||
        isFetchingProducts ||
        products.length === 0
      ) {
        return;
      }

      const threshold = 24; // px from bottom to trigger load
      if (
        target.scrollHeight - target.scrollTop - target.clientHeight <
        threshold
      ) {
        setProductsLoadMorePending(true);
        setProductOffset((prev) => prev + 10);
      }
    },
    [
      productsLoadMorePending,
      canLoadMoreProducts,
      isLoadingProducts,
      isFetchingProducts,
      products.length,
    ],
  );

  useEffect(() => {
    if (previousProductIdRef.current !== form.productId) {
      previousProductIdRef.current = form.productId;
      setVersionOffset(0);
      setVersions([]);
      setCachedVersionsTotalRecords(null);
    }

    if (!form.productId) {
      return;
    }

    if (!versionsPage) {
      return;
    }

    const pageItems = versionsPage.versions ?? [];
    const offset = versionsPage.offset ?? 0;
    const pageLimit =
      typeof versionsPage.limit === "number" && versionsPage.limit > 0
        ? versionsPage.limit
        : 10;

    const applyServerVersionsTotal = (): void => {
      if (
        typeof versionsPage.totalRecords === "number" &&
        !Number.isNaN(versionsPage.totalRecords)
      ) {
        setCachedVersionsTotalRecords(versionsPage.totalRecords);
      }
    };

    if (offset > 0 && pageItems.length === 0) {
      setCachedVersionsTotalRecords(accumulatedVersionsLengthRef.current);
      return;
    }

    if (offset === 0) {
      setVersions(pageItems);
      if (pageItems.length < pageLimit) {
        setCachedVersionsTotalRecords(pageItems.length);
      } else {
        applyServerVersionsTotal();
      }
      return;
    }

    const prevVersions = versionsRef.current;
    const prevVersionIds = new Set(
      prevVersions.map((v) => v.id).filter((id): id is string => Boolean(id)),
    );
    const newVersionItems = pageItems.filter(
      (v) =>
        typeof v.id === "string" &&
        v.id.length > 0 &&
        !prevVersionIds.has(v.id),
    );
    const mergedVersionsLen = prevVersions.length + newVersionItems.length;

    if (newVersionItems.length === 0) {
      setCachedVersionsTotalRecords(prevVersions.length);
      return;
    }

    if (pageItems.length < pageLimit) {
      setCachedVersionsTotalRecords(mergedVersionsLen);
    } else {
      applyServerVersionsTotal();
    }
    setVersions([...prevVersions, ...newVersionItems]);
  }, [form.productId, versionsPage]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const versionsTotalRecords = Number.isFinite(cachedVersionsTotalRecords)
    ? (cachedVersionsTotalRecords as number)
    : Number.isFinite(versionsPage?.totalRecords)
      ? (versionsPage!.totalRecords as number)
      : versions.length;
  const canLoadMoreVersions = versions.length < versionsTotalRecords;

  /**
   * Footer spinner only after the user scrolls to load more (pending flag), not for
   * background refetches or inflated API totalRecords.
   */
  const isFetchingMoreProducts =
    productsLoadMorePending &&
    isFetchingProducts &&
    productOffset > 0 &&
    products.length > 0;
  const isFetchingMoreVersions =
    versionsLoadMorePending &&
    isFetchingVersions &&
    versionOffset > 0 &&
    versions.length > 0;

  const handleVersionsScroll = useCallback(
    (event: UIEvent<HTMLElement>) => {
      const target = event.currentTarget;
      if (
        versionsLoadMorePending ||
        !canLoadMoreVersions ||
        isLoadingVersions ||
        isFetchingVersions ||
        versions.length === 0
      ) {
        return;
      }

      const threshold = 24;
      if (
        target.scrollHeight - target.scrollTop - target.clientHeight <
        threshold
      ) {
        setVersionsLoadMorePending(true);
        setVersionOffset((prev) => prev + 10);
      }
    },
    [
      versionsLoadMorePending,
      canLoadMoreVersions,
      isLoadingVersions,
      isFetchingVersions,
      versions.length,
    ],
  );

  const postProduct = usePostDeploymentProduct();

  const isSubmitting = postProduct.isPending;
  const isValid =
    !!form.productId &&
    !!form.versionId &&
    !!projectId &&
    deploymentId.length > 0;

  const handleProductChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const productId = event.target.value;
      setForm((prev) => ({
        ...prev,
        productId,
        versionId: "",
      }));
    },
    [],
  );

  const handleVersionChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      setForm((prev) => ({ ...prev, versionId: event.target.value }));
    },
    [],
  );

  const handleTextChange = useCallback(
    (field: "cores" | "tps" | "description") =>
      (event: ChangeEvent<HTMLInputElement>) => {
        setForm((prev) => ({ ...prev, [field]: event.target.value }));
      },
    [],
  );

  const submit = useCallback(async (): Promise<AddedProductSummary> => {
    if (!isValid) {
      throw new Error("Product and version are required.");
    }

    const selectedProduct = products.find((p) => p.id === form.productId);
    const selectedVersion = versions.find((v) => v.id === form.versionId);

    await postProduct.mutateAsync({
      deploymentId,
      body: {
        productId: form.productId,
        versionId: form.versionId,
        projectId,
        cores: parseValidNumber(form.cores),
        tps: parseValidNumber(form.tps),
        description: form.description || undefined,
      },
    });

    return {
      productId: form.productId,
      productLabel:
        selectedProduct?.label ?? selectedProduct?.name ?? form.productId,
      versionId: form.versionId,
      versionLabel: selectedVersion?.version ?? form.versionId,
    };
  }, [isValid, products, versions, form, postProduct, deploymentId, projectId]);

  return {
    form,
    products,
    versions,
    isLoadingProducts,
    isFetchingProducts,
    isLoadingVersions,
    isFetchingVersions,
    canLoadMoreProducts,
    canLoadMoreVersions,
    isFetchingMoreProducts,
    isFetchingMoreVersions,
    handleProductsScroll,
    handleVersionsScroll,
    handleProductChange,
    handleVersionChange,
    handleTextChange,
    isValid,
    isSubmitting,
    submit,
    resetForm,
  };
}
