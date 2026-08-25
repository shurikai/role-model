import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "./contexts/AuthContext";
import { RequireAuth } from "./components/RequireAuth";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { Login } from "./routes/Login";
import { Signup } from "./routes/Signup";
import { Applications } from "./routes/Applications";
import { ApplicationNew } from "./routes/ApplicationNew";
import { ApplicationDetail } from "./routes/ApplicationDetail";
import { ImportStart } from "./routes/ImportStart";
import { ImportReview } from "./routes/ImportReview";
import { CareerImportStart } from "./routes/CareerImportStart";
import { EntityDraftReview } from "./routes/EntityDraftReview";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <ErrorBoundary>
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route path="/signup" element={<Signup />} />
              <Route element={<RequireAuth />}>
                <Route
                  path="/"
                  element={<Navigate to="/applications" replace />}
                />
                <Route path="/applications" element={<Applications />} />
                <Route path="/applications/new" element={<ApplicationNew />} />
                <Route
                  path="/applications/:id"
                  element={<ApplicationDetail />}
                />
                {/*
                  Two import paths, deliberately namespaced apart. Bare
                  /import/... is the narrow one (contribution drafts against
                  existing positions); /import/career/... is the wide one that
                  drafts the employers and positions too.
                */}
                <Route path="/import/new" element={<ImportStart />} />
                <Route
                  path="/import/career/new"
                  element={<CareerImportStart />}
                />
                <Route
                  path="/import/career/:batchID"
                  element={<EntityDraftReview />}
                />
                <Route path="/import/:batchID" element={<ImportReview />} />
              </Route>
            </Routes>
          </ErrorBoundary>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
