import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "../auth-provider";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn(), post: jest.fn() } }));
jest.mock("@/lib/auth", () => ({
  getAccessToken: jest.fn(),
  getSubjectHint: jest.fn(),
  clearTokens: jest.fn(),
}));

const { api } = jest.requireMock("@/lib/api");
const { getAccessToken, getSubjectHint, clearTokens } = jest.requireMock("@/lib/auth");

function Probe() {
  const ctx = useAuth();
  return (
    <div>
      <div data-testid="state">
        {JSON.stringify({ admin: ctx.admin, subjectType: ctx.subjectType, loading: ctx.loading })}
      </div>
      <button onClick={() => ctx.signOut()}>sign out</button>
      <button onClick={() => ctx.refreshSubject()}>refresh</button>
    </div>
  );
}

describe("AuthProvider", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("no token or no hint results in unauthenticated state without an api call", async () => {
    getAccessToken.mockReturnValue(null);
    getSubjectHint.mockReturnValue(null);

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: null, subjectType: null, loading: false })
      )
    );
    expect(api.get).not.toHaveBeenCalled();
  });

  it("platform_admin hint short-circuits without calling /api/admin/me", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("platform_admin");

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: null, subjectType: "platform_admin", loading: false })
      )
    );
    expect(api.get).not.toHaveBeenCalled();
  });

  it("admin hint fetches /api/admin/me and populates admin on success", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    const adminFixture = { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" };
    api.get.mockResolvedValue({ data: { data: adminFixture } });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: adminFixture, subjectType: "admin", loading: false })
      )
    );
  });

  it("admin hint clears tokens and stays unauthenticated when /api/admin/me fails", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    api.get.mockRejectedValue(new Error("401"));

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: null, subjectType: null, loading: false })
      )
    );
    expect(clearTokens).toHaveBeenCalled();
  });

  it("signOut posts to /api/auth/logout, clears tokens, and does not reset context state", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    const adminFixture = { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" };
    api.get.mockResolvedValue({ data: { data: adminFixture } });
    api.post.mockResolvedValue({});

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"subjectType":"admin"')
    );

    await user.click(screen.getByText("sign out"));

    await waitFor(() => expect(api.post).toHaveBeenCalledWith("/api/auth/logout"));
    expect(clearTokens).toHaveBeenCalled();
    // signOut does not itself reset admin/subjectType state.
    expect(screen.getByTestId("state")).toHaveTextContent(
      JSON.stringify({ admin: adminFixture, subjectType: "admin", loading: false })
    );
  });

  it("signOut swallows a failing logout call and still clears tokens", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    api.get.mockResolvedValue({
      data: { data: { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" } },
    });
    api.post.mockRejectedValue(new Error("network"));

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"subjectType":"admin"')
    );

    await user.click(screen.getByText("sign out"));

    await waitFor(() => expect(clearTokens).toHaveBeenCalled());
    expect(api.post).toHaveBeenCalledWith("/api/auth/logout");
  });

  it("refreshSubject re-runs the loader and picks up new data", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    const first = { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" };
    const second = { id: "1", email: "a@b.com", company_slug: "other", created_at: "now" };
    api.get.mockResolvedValueOnce({ data: { data: first } }).mockResolvedValueOnce({
      data: { data: second },
    });

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"company_slug":"acme"')
    );

    await user.click(screen.getByText("refresh"));

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"company_slug":"other"')
    );
  });
});
