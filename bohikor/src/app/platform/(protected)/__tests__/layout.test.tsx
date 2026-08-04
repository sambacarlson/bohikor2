import { render, screen } from "@testing-library/react";
import PlatformLayout from "../layout";

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: jest.fn(), back: jest.fn() }),
}));

jest.mock("@/components/auth-guard", () => ({
  AuthGuard: jest.fn(({ children, loginHref }: { children: React.ReactNode; loginHref: string }) => (
    <div data-testid="guard" data-loginhref={loginHref}>
      {children}
    </div>
  )),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");

describe("PlatformLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("wraps children in AuthGuard with loginHref=/platform/login", () => {
    useAuth.mockReturnValue({ subjectType: "platform_admin" });
    render(
      <PlatformLayout>
        <div>platform content</div>
      </PlatformLayout>
    );

    const guard = screen.getByTestId("guard");
    expect(guard).toHaveAttribute("data-loginhref", "/platform/login");
  });

  it("renders children when subjectType is platform_admin", () => {
    useAuth.mockReturnValue({ subjectType: "platform_admin" });
    render(
      <PlatformLayout>
        <div>platform content</div>
      </PlatformLayout>
    );

    expect(screen.getByText("platform content")).toBeInTheDocument();
  });

  it("renders ForbiddenPage instead of children when subjectType is admin, not platform_admin", () => {
    useAuth.mockReturnValue({ subjectType: "admin" });
    render(
      <PlatformLayout>
        <div>platform content</div>
      </PlatformLayout>
    );

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("platform content")).not.toBeInTheDocument();
  });

  it("renders ForbiddenPage instead of children when subjectType is null", () => {
    useAuth.mockReturnValue({ subjectType: null });
    render(
      <PlatformLayout>
        <div>platform content</div>
      </PlatformLayout>
    );

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("platform content")).not.toBeInTheDocument();
  });
});
