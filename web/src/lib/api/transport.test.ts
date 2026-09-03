import { describe, it, expect, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { createAuthInterceptor, createSupervisorTransport } from "./transport";

describe("transport & authInterceptor", () => {
  it("creates transport with binary format enabled", () => {
    const transport = createSupervisorTransport({ baseUrl: "http://localhost:8080" });
    expect(transport).toBeDefined();
    expect(typeof transport.unary).toBe("function");
    expect(typeof transport.stream).toBe("function");
  });

  it("triggers onUnauthenticated callback on Code.Unauthenticated", async () => {
    const onUnauthenticated = vi.fn();
    const interceptor = createAuthInterceptor(onUnauthenticated);

    const mockNext = vi
      .fn()
      .mockRejectedValue(new ConnectError("unauthenticated session", Code.Unauthenticated));

    const mockReq = {
      service: {} as any,
      method: {} as any,
      url: "http://localhost:8080/supervisor.v1.AuthService/GetSession",
      init: {},
      message: {},
      stream: false,
      header: new Headers(),
    };

    await expect(interceptor(mockNext)(mockReq as any)).rejects.toThrow("unauthenticated session");
    expect(onUnauthenticated).toHaveBeenCalledTimes(1);
  });

  it("does not trigger onUnauthenticated on other error codes", async () => {
    const onUnauthenticated = vi.fn();
    const interceptor = createAuthInterceptor(onUnauthenticated);

    const mockNext = vi
      .fn()
      .mockRejectedValue(new ConnectError("invalid argument", Code.InvalidArgument));

    const mockReq = {
      service: {} as any,
      method: {} as any,
      url: "http://localhost:8080/supervisor.v1.PoolService/CreatePool",
      init: {},
      message: {},
      stream: false,
      header: new Headers(),
    };

    await expect(interceptor(mockNext)(mockReq as any)).rejects.toThrow("invalid argument");
    expect(onUnauthenticated).not.toHaveBeenCalled();
  });
});
