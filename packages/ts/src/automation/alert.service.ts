import { Injectable, Logger } from '@nestjs/common';

@Injectable()
export class AlertService {
  private readonly logger = new Logger(AlertService.name);

  async triggerAlert(title: string, message: string, severity: 'INFO' | 'WARNING' | 'CRITICAL' = 'WARNING'): Promise<boolean> {
    const webhookUrl = process.env.ALERT_WEBHOOK_URL;
    if (!webhookUrl) {
      this.logger.warn(`Alert triggered but no ALERT_WEBHOOK_URL is configured: [${severity}] ${title} - ${message}`);
      return false;
    }

    try {
      const payload = {
        username: 'System Alert Bot',
        content: `**[${severity}] ${title}**\n${message}\nTimestamp: ${new Date().toISOString()}`,
      };

      const response = await fetch(webhookUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      this.logger.log(`Alert alert dispatched successfully: ${title}`);
      return true;
    } catch (err) {
      this.logger.error(`Failed to dispatch automated webhook alert`, err.stack);
      return false;
    }
  }
}
