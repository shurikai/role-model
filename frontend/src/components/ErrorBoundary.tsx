import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { Link } from "react-router-dom";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

// A render error anywhere below this point would otherwise unmount the whole
// React root and leave a blank page with nothing in the UI to explain it.
// Must be a class component — React has no hook equivalent.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("Unhandled render error:", error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) {
      return this.props.children;
    }

    return (
      <div className="mx-auto mt-12 max-w-3xl px-4">
        <div className="rounded border border-red-300 bg-red-50 p-4">
          <h1 className="mb-1 text-lg font-semibold text-red-800">
            Something went wrong
          </h1>
          <p className="text-sm text-red-700">{error.message}</p>
          <p className="mt-3 text-sm">
            <Link to="/applications" className="text-red-800 underline">
              Back to applications
            </Link>
          </p>
        </div>
      </div>
    );
  }
}
