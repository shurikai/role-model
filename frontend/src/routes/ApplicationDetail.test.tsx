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
    preference_score: 33.33,
    preference_gaps: ["remote-first"],
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
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
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
                sentiment: "hard_exclude",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      const button = await screen.findByRole("button", { name: "Generate Resume" });
      expect(button).toBeEnabled();
      expect(
        screen.queryByText(/Run the fit evaluation first to see scores/),
      ).not.toBeInTheDocument();
    });

    it("disables Generate Resume when no fit report exists", async () => {
      vi.stubGlobal("fetch", stubFetch(makeApplication(), []));
      renderDetail();

      const button = await screen.findByRole("button", { name: "Generate Resume" });
      expect(button).toBeDisabled();
      expect(
        await screen.findByText(/Run the fit evaluation first to see scores/),
      ).toBeInTheDocument();
    });
  });

  describe("fit report rendering", () => {
    it("renders scores and narrative even when anti_pattern_passed is false", async () => {
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
                sentiment: "hard_exclude",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      expect(await screen.findByText("This is the narrative.")).toBeInTheDocument();
      expect(screen.getByText("Technical score:")).toBeInTheDocument();
      expect(screen.getByText("Preference score:")).toBeInTheDocument();
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
                sentiment: "hard_exclude",
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

      expect(await screen.findByText("This is the narrative.")).toBeInTheDocument();
      expect(screen.queryByText("Anti-pattern flag")).not.toBeInTheDocument();
      // The old always-on "passed" badge is gone too.
      expect(screen.queryByText(/Hard gate/)).not.toBeInTheDocument();
    });

    // Reports written before the gate stopped blocking returned early, so they
    // carry no scores and no narrative. Those rows are still in the database.
    it("shows a dash rather than a bare /100 for a legacy report with null scores", async () => {
      vi.stubGlobal(
        "fetch",
        stubFetch(makeApplication(), [
          makeFitReport({
            anti_pattern_passed: false,
            technical_score: null,
            preference_score: null,
            preference_gaps: null,
            narrative: null,
            anti_pattern_hits: [
              {
                id: "pref-1",
                label: "IT consulting / staff augmentation model",
                preference_type: "anti_pattern",
                sentiment: "hard_exclude",
                notes: null,
              },
            ],
          }),
        ]),
      );
      renderDetail();

      expect(await screen.findByText("Anti-pattern flag")).toBeInTheDocument();
      expect(screen.queryByText(/\/100/)).not.toBeInTheDocument();
      expect(screen.getByText("Technical score:").closest("p")).toHaveTextContent(
        "Technical score: —",
      );
      expect(screen.getByText("Preference score:").closest("p")).toHaveTextContent(
        "Preference score: —",
      );
    });
  });

  describe("screening summary", () => {
    it("renders from jd_signals once extraction has run", async () => {
      vi.stubGlobal("fetch", stubFetch(makeApplication(), []));
      renderDetail();

      expect(await screen.findByText("Screening summary")).toBeInTheDocument();
      expect(screen.getByText("Huntsville, AL")).toBeInTheDocument();
      expect(screen.getByText("active TS/SCI required")).toBeInTheDocument();
      expect(screen.getByText("military-coded language throughout")).toBeInTheDocument();
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
