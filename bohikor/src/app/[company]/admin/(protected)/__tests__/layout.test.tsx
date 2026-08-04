import { render, screen } from "@testing-library/react";
import CompanyAdminLayout from "../layout";

jest.mock("next/navigation", () => ({
  useParams: () => ({ company: "acme" }),
  usePathname: () => "/acme/admin",
  useRouter: () => ({ push: jest.fn(), back: jest.fn() }),
}));

jest.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");

describe("CompanyAdminLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders ForbiddenPage when there is no admin in context (e.g. wrong subject type)", () => {
    useAuth.mockReturnValue({ admin: null, signOut: jest.fn() });
    render(
      <CompanyAdminLayout>
        <div>dashboard content</div>
      </CompanyAdminLayout>
    );

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("dashboard content")).not.toBeInTheDocument();
  });

  it("renders the Sidebar and children when an admin is present in context", () => {
    useAuth.mockReturnValue({ admin: { id: "1", email: "a@b.com" }, signOut: jest.fn() });
    render(
      <CompanyAdminLayout>
        <div>dashboard content</div>
      </CompanyAdminLayout>
    );

    expect(screen.getByText("dashboard content")).toBeInTheDocument();
    expect(screen.getByText("Dashboard")).toBeInTheDocument(); // sidebar nav item
  });
});
