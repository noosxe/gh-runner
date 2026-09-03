import {
  Code,
  ConnectError,
  createClient,
  type Interceptor,
  type Transport,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  AuthService,
  PoolService,
  AuthProfileService,
  OnboardingService,
  AnalyticsService,
  LogService,
  RenovateService,
  ImageUpdateService,
} from "../../gen/api_pb";

export interface TransportOptions {
  baseUrl?: string;
  onUnauthenticated?: () => void;
}

export function createAuthInterceptor(onUnauthenticated?: () => void): Interceptor {
  return (next) => async (req) => {
    try {
      return await next(req);
    } catch (err) {
      if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
        if (onUnauthenticated) {
          onUnauthenticated();
        } else if (
          typeof window !== "undefined" &&
          !window.location.pathname.startsWith("/login") &&
          !window.location.pathname.startsWith("/onboarding")
        ) {
          const redirect = encodeURIComponent(window.location.pathname);
          window.location.href = `/login?redirect=${redirect}`;
        }
      }
      throw err;
    }
  };
}

export function createSupervisorTransport(options: TransportOptions = {}): Transport {
  const baseUrl =
    options.baseUrl ??
    (typeof window !== "undefined" ? window.location.origin : "http://localhost:8080");

  return createConnectTransport({
    baseUrl,
    useBinaryFormat: true,
    interceptors: [createAuthInterceptor(options.onUnauthenticated)],
    fetch: (input, init) =>
      fetch(input, {
        ...init,
        credentials: "same-origin",
      }),
  });
}

// Default singleton transport for browser application
export const defaultTransport = createSupervisorTransport();

// Service clients bound to default transport
export const authClient = createClient(AuthService, defaultTransport);
export const poolClient = createClient(PoolService, defaultTransport);
export const authProfileClient = createClient(AuthProfileService, defaultTransport);
export const onboardingClient = createClient(OnboardingService, defaultTransport);
export const analyticsClient = createClient(AnalyticsService, defaultTransport);
export const logClient = createClient(LogService, defaultTransport);
export const renovateClient = createClient(RenovateService, defaultTransport);
export const imageClient = createClient(ImageUpdateService, defaultTransport);
