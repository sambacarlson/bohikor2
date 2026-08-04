import { render } from "@testing-library/react";
import CompanyLayout from "../layout";

const mockReplace = jest.fn();

jest.mock("next/navigation", () => ({
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ replace: mockReplace }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");

describe("CompanyLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("does not redirect while loading", () => {
    useAuth.mockReturnValue({ admin: null, subjectType: "admin", loading: true });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("does not redirect for a non-admin subjectType", () => {
    useAuth.mockReturnValue({ admin: null, subjectType: "platform_admin", loading: false });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("does not redirect when the admin's company_slug matches the URL param", () => {
    useAuth.mockReturnValue({
      admin: { company_slug: "acme" },
      subjectType: "admin",
      loading: false,
    });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("redirects to the admin's own company when the slug mismatches", () => {
    useAuth.mockReturnValue({
      admin: { company_slug: "other" },
      subjectType: "admin",
      loading: false,
    });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).toHaveBeenCalledWith("/other/admin");
  });
});
