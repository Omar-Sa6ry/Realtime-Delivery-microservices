import { Injectable, Logger } from '@nestjs/common';

export type AlertSeverity = 'INFO' | 'WARNING' | 'CRITICAL';

const SEVERITY_LABEL: Record<AlertSeverity, string> = {
  INFO: 'info',
  WARNING: 'warning',
  CRITICAL: 'critical',
};

const REQUEST_TIMEOUT_MS = 5000;

@Injectable()
export class AlertService {
  private readonly logger = new Logger(AlertService.name);

  async triggerAlert(
    title: string,
    message: string,
    severity: AlertSeverity = 'WARNING',
    service?: string,
  ): Promise<boolean> {
    const alertmanagerUrl = (process.env.ALERTMANAGER_URL || '').replace(
      /\/+$/,
      '',
    );
    if (!alertmanagerUrl) {
      this.logger.warn(
        `Alert triggered but no ALERTMANAGER_URL is configured: [${severity}] ${title} - ${message}`,
      );
      return false;
    }

    const payload = [
      {
        labels: {
          alertname: 'ApplicationAlert',
          severity: SEVERITY_LABEL[severity] ?? 'warning',
          source: 'application',
          service: service || process.env.SERVICE_NAME || 'application',
          title,
        },
        annotations: {
          summary: title,
          description: message,
        },
        startsAt: new Date().toISOString(),
      },
    ];

    try {
      const response = await fetch(`${alertmanagerUrl}/api/v2/alerts`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      this.logger.log(`Alert dispatched to Alertmanager: ${title}`);
      return true;
    } catch (err: any) {
      this.logger.error(
        `Failed to dispatch automated alert to Alertmanager`,
        err?.stack ?? String(err),
      );
      return false;
    }
  }
}
