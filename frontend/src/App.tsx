import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "./contexts/AuthContext";
import { RequireAuth } from "./components/RequireAuth";
import { Login } from "./routes/Login";
import { Signup } from "./routes/Signup";
import { Applications } from "./routes/Applications";
import { ApplicationNew } from "./routes/ApplicationNew";
import { ApplicationDetail } from "./routes/ApplicationDetail";

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
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/signup" element={<Signup />} />
            <Route element={<RequireAuth />}>
              <Route path="/" element={<Navigate to="/applications" replace />} />
              <Route path="/applications" element={<Applications />} />
              <Route path="/applications/new" element={<ApplicationNew />} />
              <Route path="/applications/:id" element={<ApplicationDetail />} />
            </Route>
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
