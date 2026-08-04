import { render, screen } from "@testing-library/react";
import LandingPage from "../page";
import EmployeeHomePage from "../[company]/page";
import EmployeeLoginPage from "../[company]/login/page";
import SignupPage from "../[company]/signup/page";
import VerifyPage from "../[company]/verify/page";
import CreatePinPage from "../[company]/create-pin/page";
import ForgotPinPage from "../[company]/forgot-pin/page";
import ResetPinPage from "../[company]/reset-pin/page";
import HistoryPage from "../[company]/history/page";
import AccountPage from "../[company]/account/page";
import EventsPage from "../[company]/admin/(protected)/events/page";
import BalancePage from "../[company]/admin/(protected)/balance/page";
import PlatformConsolePage from "../platform/(protected)/page";

const stubs: [string, () => React.ReactElement, string][] = [
  ["LandingPage", LandingPage, "Sign in — coming soon."],
  ["EmployeeHomePage", EmployeeHomePage, "Employee home — coming soon."],
  ["EmployeeLoginPage", EmployeeLoginPage, "Employee sign in — coming soon."],
  ["SignupPage", SignupPage, "Sign up — coming soon."],
  ["VerifyPage", VerifyPage, "Verify — coming soon."],
  ["CreatePinPage", CreatePinPage, "Create PIN — coming soon."],
  ["ForgotPinPage", ForgotPinPage, "Forgot PIN — coming soon."],
  ["ResetPinPage", ResetPinPage, "Reset PIN — coming soon."],
  ["HistoryPage", HistoryPage, "History — coming soon."],
  ["AccountPage", AccountPage, "Account — coming soon."],
  ["EventsPage", EventsPage, "Events — coming soon."],
  ["BalancePage", BalancePage, "Balance & ledger — coming soon."],
  ["PlatformConsolePage", PlatformConsolePage, "Platform console — coming soon."],
];

describe.each(stubs)("%s", (_name, Component, expectedText) => {
  it(`renders "${expectedText}"`, () => {
    render(<Component />);
    expect(screen.getByText(expectedText)).toBeInTheDocument();
  });
});
