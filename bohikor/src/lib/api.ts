"use client";

import axios from "axios";
import { getAccessToken, getRefreshToken, setTokens, clearTokens } from "./auth";

const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

// Where to send the user when a 401 can't be recovered (no refresh token, or
// the refresh call itself fails). `/` is a static placeholder page with no
// login form, so route back to the login page for whichever area of the app
// the user was in rather than stranding them there.
function reauthRedirectPath(): string {
  const { pathname } = window.location;
  if (pathname.startsWith("/platform")) return "/platform/login";
  const [, company, section] = pathname.split("/");
  if (!company) return "/";
  if (section === "admin") return `/${company}/admin/login`;
  return `/${company}/login`;
}

let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

function onRefreshed(token: string) {
  refreshSubscribers.forEach((cb) => cb(token));
  refreshSubscribers = [];
}

function addRefreshSubscriber(cb: (token: string) => void) {
  refreshSubscribers.push(cb);
}

api.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      if (!isRefreshing) {
        isRefreshing = true;

        try {
          const refreshToken = getRefreshToken();
          if (!refreshToken) {
            clearTokens();
            isRefreshing = false;
            if (typeof window !== "undefined") {
              window.location.href = reauthRedirectPath();
            }
            return Promise.reject(error);
          }

          const response = await axios.post(
            `${api.defaults.baseURL}/api/auth/refresh`,
            { refresh_token: refreshToken }
          );

          const { access_token, refresh_token: newRefreshToken } = response.data.data;
          setTokens(access_token, newRefreshToken);

          onRefreshed(access_token);
          isRefreshing = false;

          originalRequest.headers.Authorization = `Bearer ${access_token}`;
          return api(originalRequest);
        } catch {
          clearTokens();
          isRefreshing = false;
          if (typeof window !== "undefined") {
            window.location.href = reauthRedirectPath();
          }
          return Promise.reject(error);
        }
      }

      return new Promise((resolve) => {
        addRefreshSubscriber((token: string) => {
          originalRequest.headers.Authorization = `Bearer ${token}`;
          resolve(api(originalRequest));
        });
      });
    }

    return Promise.reject(error);
  }
);

export interface ApiError {
  error?: string;
}

export function getApiErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as ApiError | undefined;
    if (data?.error) return data.error;
    if (error.response?.status === 401) return "Session expired. Please log in again.";
  }
  return "Something went wrong. Please try again.";
}

export { api };