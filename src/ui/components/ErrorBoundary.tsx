import { Component, type ReactNode } from 'react';
import { t } from '../../i18n/i18n';

interface State { error: Error | null }

export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  override state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  override componentDidCatch(error: Error): void {
    console.error(error);
  }

  override render(): ReactNode {
    if (this.state.error) {
      return (
        <div className="panel p-4 m-4">
          <h2 className="text-lg text-bad mb-2">{t('errorBoundary.title')}</h2>
          <pre className="text-[11px] whitespace-pre-wrap bg-panel-dark p-2 rounded">{this.state.error.message}\n{this.state.error.stack}</pre>
          <p className="mt-2 text-[12px]">{t('errorBoundary.safe')}</p>
          <button className="btn mt-2" onClick={() => location.reload()}>{t('errorBoundary.reload')}</button>
        </div>
      );
    }
    return this.props.children;
  }
}
