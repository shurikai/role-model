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
      <div className="flex min-h-screen flex-col bg-paper">
        <div className="mx-auto w-full max-w-3xl px-6 py-20">
          <p className="mb-2 font-mono text-[11px] tracking-widest text-reject uppercase">
            Error
          </p>
          <h1 className="mb-1 font-display text-2xl font-bold text-ink">
            Something went wrong
          </h1>
          <p className="mb-4 border border-reject bg-card p-4 font-body text-sm text-ink">
            {error.message}
          </p>
          <Link
            to="/applications"
            className="font-body text-sm text-ink-dim underline"
          >
            Back to applications
          </Link>
        </div>
      </div>
    );
  }
}
