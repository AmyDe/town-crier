import { Component, type ErrorInfo, type ReactNode } from 'react';
import { appInsights } from '../telemetry';
import styles from './ErrorBoundary.module.css';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    appInsights?.trackException({
      exception: error,
      properties: { componentStack: errorInfo.componentStack ?? '' },
    });
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className={styles.container}>
          <h1 className={styles.title}>Something went wrong</h1>
          <p className={styles.message}>
            The application encountered an unexpected error. Please try
            refreshing the page.
          </p>
          <button
            className={styles.button}
            onClick={() => window.location.reload()}
          >
            Refresh
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
