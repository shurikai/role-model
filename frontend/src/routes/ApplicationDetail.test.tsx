import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApplicationDetail } from "./ApplicationDetail";
import type { Application, FitReport, ScreeningSummary } from "../lib/types";

const APP_ID = "app-1";

const screeningSummary: ScreeningSummary = {
  location: "Huntsville, AL",
  work_arrangement: "fully onsite",
  travel: "approximately 25%",
  industry: "defense/autonomous systems",
  clearance_citizenship: "active TS/SCI required",
  other_flags: ["military-coded language throughout"],
};

function makeApplication(overrides: Partial<Application> = {}): Application {
  return {
    id: APP_ID,
    user_id: "u1",
    company_name: "Arclight",
    role_title: "Principal Engineer",
    jd_url: null,
    jd_text: "some jd text",
    jd_signals: {
      required_skills: ["Go"],
      preferred_skills: [],
      seniority: "principal",
      domain: "defense",
      work_type: "onsite",
      culture_signals: [],
      screening_summary: screeningSummary,
    },
    status: "draft",
    applied_on: null,
    notes: null,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

function makeFitReport(overrides: Partial<FitReport> = {}): FitReport {
  return {
    id: "fit-1",
    user_id: "u1",
    application_id: APP_ID,
    anti_pattern_passed: true,
    anti_pattern_hits: null,
    technical_score: 100,
    technical_gaps: null,
    technical_partial: null,
    preference_matches: [
      {
        id: "pref-match-1",
        label: "distributed systems",
        preference_type: "domain",
        sentiment: "positive",
        notes: null,
      },
    ],
    preference_gaps: [
      {
        id: "pref-gap-1",
        label: "remote-first",
        preference_type: "culture",
        sentiment: "positive",
        notes: null,
      },
    ],
    preference_conflicts: null,
    narrative: "This is the narrative.",
    screening_summary: screeningSummary,
    created_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

/**
 * Routes the three GETs ApplicationDetail issues on mount. Order matters less
 * than specificity: /fit and /versions must be checked before the bare
 * application path, since all three share the same prefix.
 */
function stubFetch(application: Application, fitReports: FitReport[]) {
  return vi.fn(async (url: unknown) => {
    const path = String(url);
    let body: unknown = null;
    if (path.includes("/fit")) {
      body = fitReports;
    } else if (path.includes("/versions")) {
      body = [];
    } else {
      body = application;
    }
    return {
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(body),
    } as unknown as Response;
  });
}

function renderDetail() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/applications/${APP_ID}`]}>
        <Routes>
          <Route path="/applications/:id" element={<ApplicationDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ApplicationDetail", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    cleanup();
  });

  describe("generation gating", () => {
    it("enables Generate Resume when a fit report exists and the anti-pattern check failed", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            anti_pattern_passed: false,
            anti_pattern_hits: [
              {
                id: "pref-1",
                label: "defense / aerospace",
                preference_type: "domain",
                sentiment: "negative",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      const button = await screen.findByRole("button", {
        name: "Generate Resume",
      });
      expect(button).toBeEnabled();
      expect(
        screen.queryByText(/Run the fit evaluation first to see scores/),
      ).not.toBeInTheDocument();
    });

    it("disables Generate Resume when no fit report exists", async () => {
      vi.stubGlobal("fetch", stubFetch(makeApplication(), []));
      renderDetail();

      const button = await screen.findByRole("button", {
        name: "Generate Resume",
      });
      expect(button).toBeDisabled();
      expect(
        await screen.findByText(/Run the fit evaluation first to see scores/),
      ).toBeInTheDocument();
    });
  });

  describe("fit report rendering", () => {
    it("renders the preference lists and narrative even when anti_pattern_passed is false", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            anti_pattern_passed: false,
            anti_pattern_hits: [
              {
                id: "pref-1",
                label: "defense / aerospace",
                preference_type: "domain",
                sentiment: "negative",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      expect(
        await screen.findByText("This is the narrative."),
      ).toBeInTheDocument();
      expect(screen.getByText("Technical score:")).toBeInTheDocument();
      // Preference fit is four lists and no score. Gaps and matches render as
      // their own labelled lists; the hard-gate hits are the anti-pattern
      // callout above and are deliberately not repeated down here.
      expect(screen.queryByText("Preference score:")).not.toBeInTheDocument();
      expect(screen.getByText("Preferences matched:")).toBeInTheDocument();
      expect(screen.getByText("distributed systems")).toBeInTheDocument();
      expect(
        screen.getByText("Preferences not mentioned:"),
      ).toBeInTheDocument();
      expect(screen.getByText("remote-first")).toBeInTheDocument();
    });

    it("shows the anti-pattern hit by label, not as a raw object", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            anti_pattern_passed: false,
            anti_pattern_hits: [
              {
                id: "pref-1",
                label: "defense / aerospace",
                preference_type: "domain",
                sentiment: "negative",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      expect(await screen.findByText("Anti-pattern flag")).toBeInTheDocument();
      expect(screen.getByText("defense / aerospace")).toBeInTheDocument();
      expect(screen.queryByText(/\[object Object\]/)).not.toBeInTheDocument();
    });

    it("omits the anti-pattern section entirely when there are no hits", async () => {
      vi.stubGlobal("fetch", stubFetch(makeApplication(), [makeFitReport()]));
      renderDetail();

      expect(
        await screen.findByText("This is the narrative."),
      ).toBeInTheDocument();
      expect(screen.queryByText("Anti-pattern flag")).not.toBeInTheDocument();
      // The old always-on "passed" badge is gone too.
      expect(screen.queryByText(/Hard gate/)).not.toBeInTheDocument();
    });

    // Reports written before the gate stopped blocking returned early, so they
    // carry no scores and no narrative. Those rows are still in the database.
    it("shows a dash rather than a bare /100 for a legacy report with a null technical score", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            anti_pattern_passed: false,
            technical_score: null,
            preference_matches: null,
            preference_gaps: null,
            narrative: null,
            anti_pattern_hits: [
              {
                id: "pref-1",
                label: "IT consulting / staff augmentation model",
                preference_type: "anti_pattern",
                sentiment: "negative",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      expect(await screen.findByText("Anti-pattern flag")).toBeInTheDocument();
      expect(screen.queryByText(/\/100/)).not.toBeInTheDocument();
      expect(
        screen.getByText("Technical score:").closest("p"),
      ).toHaveTextContent("Technical score: —");
      // An empty preference list renders as nothing at all rather than as a
      // dash. There is no score to be absent — a heading with no entries under
      // it would be the only thing pretending otherwise.
      expect(
        screen.queryByText("Preferences not mentioned:"),
      ).not.toBeInTheDocument();
    });

    // A partial match is a third verdict: the skill is present, below the
    // depth the posting asked for. It must not render as a gap (which would
    // say the profile lacks it) and must name the bar, since the requirement
    // alone doesn't say what is short about it.
    it("renders a partial match with the level it fell short of", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            technical_gaps: ["GraphQL"],
            technical_partial: [
              {
                requirement: "Kafka",
                kind: "direct",
                evidence: ["Kafka"],
                required_level: "expert",
                evidence_level: "novice",
                level_signal: "expert-level Kafka",
              },
            ],
          }),
        ]),
      );
      renderDetail();

      expect(
        await screen.findByText("Below the level asked for:"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/posting asks for expert, yours is novice/),
      ).toBeInTheDocument();
      // Present, not missing: it must not have been filed under gaps.
      expect(
        screen.getByText("Technical gaps:").closest("div"),
      ).not.toHaveTextContent("Kafka");
    });

    it("omits the partial section entirely when nothing is partial", async () => {
      vi.stubGlobal("fetch", stubFetch(makeApplication(), [makeFitReport()]));
      renderDetail();

      expect(
        await screen.findByText("This is the narrative."),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("Below the level asked for:"),
      ).not.toBeInTheDocument();
    });

    // Gaps and conflicts were bare label strings before preference fit became
    // a hit list, and reports written then are still in the database. They must
    // still render their labels rather than a row of empty bullets.
    it("renders a legacy report whose preference lists hold bare strings", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            preference_matches: null,
            preference_gaps: ["remote-first", "logistics"],
            preference_conflicts: ["on-call heavy"],
          }),
        ]),
      );
      renderDetail();

      expect(await screen.findByText("remote-first")).toBeInTheDocument();
      expect(screen.getByText("logistics")).toBeInTheDocument();
      expect(screen.getByText("on-call heavy")).toBeInTheDocument();
      expect(screen.queryByText(/\[object Object\]/)).not.toBeInTheDocument();
    });
  });

  describe("screening summary", () => {
    it("renders from jd_signals once extraction has run", async () => {
      vi.stubGlobal("fetch", stubFetch(makeApplication(), []));
      renderDetail();

      expect(await screen.findByText("Screening summary")).toBeInTheDocument();
      expect(screen.getByText("Huntsville, AL")).toBeInTheDocument();
      expect(screen.getByText("active TS/SCI required")).toBeInTheDocument();
      expect(
        screen.getByText("military-coded language throughout"),
      ).toBeInTheDocument();
    });

    it("is absent, not an empty box, when screening_summary is null", async () => {
      const application = makeApplication();
      const app = {
        ...application,
        jd_signals: { ...application.jd_signals!, screening_summary: null },
      };
      vi.stubGlobal("fetch", stubFetch(app, []));
      renderDetail();

      // Signals themselves still render, so this isn't a false negative.
      expect(await screen.findByText("principal")).toBeInTheDocument();
      expect(screen.queryByText("Screening summary")).not.toBeInTheDocument();
    });
  });
});
