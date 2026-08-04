import { render, screen } from "@testing-library/react";
import { AuthGuard } from "../auth-guard";

const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");

describe("AuthGuard", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("shows only a spinner while loading, does not push, does not render children", () => {
    useAuth.mockReturnValue({ subjectType: null, loading: true });

    const { container } = render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(screen.queryByText("secret content")).not.toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("redirects to loginHref and renders nothing when unauthenticated", () => {
    useAuth.mockReturnValue({ subjectType: null, loading: false });

    render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(screen.queryByText("secret content")).not.toBeInTheDocument();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });

  it("renders children and does not redirect when authenticated", () => {
    useAuth.mockReturnValue({ subjectType: "admin", loading: false });

    render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(screen.getByText("secret content")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("redirects only after transitioning from authenticated to unauthenticated", () => {
    useAuth.mockReturnValue({ subjectType: "admin", loading: false });
    const { rerender: rerenderComponent } = render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );
    expect(mockPush).not.toHaveBeenCalled();

    useAuth.mockReturnValue({ subjectType: null, loading: false });
    rerenderComponent(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });
});
