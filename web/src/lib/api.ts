/**
 * Typed API client for the protou backend.
 *
 * All functions read PUBLIC_API_URL from the environment, which is set at
 * build time by Cloudflare Pages (or locally via .env).
 */

const API_URL =
  (import.meta.env.PUBLIC_API_URL as string | undefined) ??
  "http://localhost:8080";

// ─── Types ────────────────────────────────────────────────────────────────────

export interface Store {
  id: number;
  name: string;
  base_url: string;
}

export interface Category {
  id: number;
  name: string;
  slug: string;
  parent_id: number | null;
  icon_url: string | null;
}

export interface Listing {
  id: number;
  store: Pick<Store, "id" | "name">;
  name: string;
  description: string | null;
  price_cop: number | null;
  image_url: string | null;
  product_url: string;
  stock_signal: "in_stock" | "out_of_stock" | "unknown" | "price_on_request";
  category: Pick<Category, "id" | "name" | "slug"> | null;
  last_scraped_at: string | null;
}

export interface PaginatedListings {
  listings: Listing[];
  total: number;
  page: number;
}

export interface User {
  id: number;
  email: string;
  name: string;
  phone: string | null;
}

export interface Address {
  id: number;
  label: string | null;
  full_address: string;
  reference: string | null;
  lat: number | null;
  lng: number | null;
}

export interface CartItem {
  id: number;
  listing: Listing;
  quantity: number;
  unavailable: boolean;
}

export interface Cart {
  items: CartItem[];
  subtotal_cop: number;
}

export interface Order {
  id: number;
  status: string;
  subtotal_cop: number;
  delivery_fee_cop: number;
  total_cop: number;
  payment_method: string;
  notes: string | null;
  created_at: string;
}

export interface DeliveryFeeResponse {
  fee: number;
  breakdown: string;
  coverage_ok: boolean;
  coverage_error: string | null;
}

export interface APIError {
  error: string;
  code: string;
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_URL}${path}`;
  const res = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  });

  if (!res.ok) {
    const err: APIError = await res.json().catch(() => ({
      error: "Unknown error",
      code: "UNKNOWN",
    }));
    throw new Error(`${err.code}: ${err.error}`);
  }

  return res.json() as Promise<T>;
}

function authHeaders(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

// ─── Catalog ──────────────────────────────────────────────────────────────────

export async function searchListings(params: {
  q?: string;
  category?: string;
  store?: string;
  page?: number;
  per_page?: number;
}): Promise<PaginatedListings> {
  const qs = new URLSearchParams();
  if (params.q) qs.set("q", params.q);
  if (params.category) qs.set("category", params.category);
  if (params.store) qs.set("store", params.store);
  if (params.page) qs.set("page", String(params.page));
  if (params.per_page) qs.set("per_page", String(params.per_page));
  return request<PaginatedListings>(`/v1/listings?${qs}`);
}

export async function getListing(id: number): Promise<{ listing: Listing }> {
  return request<{ listing: Listing }>(`/v1/listings/${id}`);
}

export async function getCategories(): Promise<{ categories: Category[] }> {
  return request<{ categories: Category[] }>("/v1/categories");
}

export async function getStores(): Promise<{ stores: Store[] }> {
  return request<{ stores: Store[] }>("/v1/stores");
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

export async function register(body: {
  email: string;
  phone?: string;
  name: string;
  password: string;
}): Promise<{ user: User; token: string }> {
  return request<{ user: User; token: string }>("/v1/auth/register", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function login(body: {
  email: string;
  password: string;
}): Promise<{ user: User; token: string }> {
  return request<{ user: User; token: string }>("/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

// ─── Cart ─────────────────────────────────────────────────────────────────────

export async function getCart(token: string): Promise<{ cart: Cart }> {
  return request<{ cart: Cart }>("/v1/cart", {
    headers: authHeaders(token),
  });
}

export async function updateCartItem(
  token: string,
  listingId: number,
  quantity: number
): Promise<{ cart: Cart }> {
  return request<{ cart: Cart }>(`/v1/cart/items/${listingId}`, {
    method: "PUT",
    headers: authHeaders(token),
    body: JSON.stringify({ quantity }),
  });
}

export async function clearCart(token: string): Promise<void> {
  await request<void>("/v1/cart", {
    method: "DELETE",
    headers: authHeaders(token),
  });
}

// ─── Checkout & Orders ────────────────────────────────────────────────────────

export async function getDeliveryFee(
  token: string,
  body: { address_id?: number; address?: { full_address: string } }
): Promise<DeliveryFeeResponse> {
  return request<DeliveryFeeResponse>("/v1/checkout/delivery-fee", {
    method: "POST",
    headers: authHeaders(token),
    body: JSON.stringify(body),
  });
}

export async function createOrder(
  token: string,
  body: {
    address_id?: number;
    address?: { full_address: string; reference?: string };
    payment_method: "nequi" | "daviplata" | "efectivo" | "llaves_breve";
    notes?: string;
  }
): Promise<{ order: Order }> {
  return request<{ order: Order }>("/v1/orders", {
    method: "POST",
    headers: authHeaders(token),
    body: JSON.stringify(body),
  });
}

export async function getOrders(token: string): Promise<{ orders: Order[] }> {
  return request<{ orders: Order[] }>("/v1/orders", {
    headers: authHeaders(token),
  });
}
