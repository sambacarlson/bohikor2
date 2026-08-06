import { render, screen } from "@testing-library/react";
import EmployeeProtectedLayout from "../layout";

jest.mock("next/navigation", () => ({
  useParams: () => ({ company: "acme" }),
  usePathname: () => "/acme",
  useRouter: () => ({ push: jest.fn() }),
}));

jest.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/hooks/use-media-query", () => ({
  useMediaQuery: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { useMediaQuery } = jest.requireMock("@/hooks/use-media-query");

describe("EmployeeProtectedLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders ForbiddenPage when the subject is not a user", () => {
    useAuth.mockReturnValue({ user: null, subjectType: "admin", signOut: jest.fn() });
    useMediaQuery.mockReturnValue(false);
    render(
      <EmployeeProtectedLayout>
        <div>home content</div>
      </EmployeeProtectedLayout>
    );

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("home content")).not.toBeInTheDocument();
  });

  it("renders the mobile header and children below the lg breakpoint", () => {
    useAuth.mockReturnValue({
      user: { id: "1" },
      subjectType: "user",
      signOut: jest.fn(),
    });
    useMediaQuery.mockReturnValue(false);
    render(
      <EmployeeProtectedLayout>
        <div>home content</div>
      </EmployeeProtectedLayout>
    );

    expect(screen.getByText("Bohikor")).toBeInTheDocument();
    expect(screen.getByText("home content")).toBeInTheDocument();
  });

  it("renders the desktop sidebar and children at the lg breakpoint", () => {
    useAuth.mockReturnValue({
      user: { id: "1" },
      subjectType: "user",
      signOut: jest.fn(),
    });
    useMediaQuery.mockReturnValue(true);
    render(
      <EmployeeProtectedLayout>
        <div>home content</div>
      </EmployeeProtectedLayout>
    );

    expect(screen.getByLabelText("Home")).toBeInTheDocument();
    expect(screen.getByLabelText("History")).toBeInTheDocument();
    expect(screen.getByLabelText("Account")).toBeInTheDocument();
    expect(screen.getByText("home content")).toBeInTheDocument();
  });
});
