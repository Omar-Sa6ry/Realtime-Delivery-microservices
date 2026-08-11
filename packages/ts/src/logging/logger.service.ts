import { Injectable, LoggerService } from '@nestjs/common';
import { LoggerContext } from './logger.context';

@Injectable()
export class StructuredLogger implements LoggerService {
  private getLevelColor(level: string): string {
    switch (level.toUpperCase()) {
      case 'ERROR':
        return '\x1b[91m\x1b[1m'; // Bold Bright Red
      case 'WARN':
        return '\x1b[93m\x1b[1m'; // Bold Bright Yellow
      case 'INFO':
        return '\x1b[92m\x1b[1m'; // Bold Bright Green
      case 'DEBUG':
        return '\x1b[95m\x1b[1m'; // Bold Bright Magenta
      case 'VERBOSE':
        return '\x1b[90m\x1b[1m'; // Bold Gray
      default:
        return '\x1b[97m\x1b[1m'; // Bold White
    }
  }

  private getContextColor(context: string): string {
    const ctxLower = context.toLowerCase();
    
    // Dedicated color styles for specific components/services
    if (ctxLower.includes('alert') || ctxLower.includes('health') || ctxLower.includes('automation')) {
      return '\x1b[38;5;208m'; // Premium Orange
    }
    if (ctxLower.includes('metrics') || ctxLower.includes('prometheus') || ctxLower.includes('interceptor')) {
      return '\x1b[38;5;135m'; // Premium Purple/Violet
    }
    if (ctxLower.includes('logger') || ctxLower.includes('logging')) {
      return '\x1b[38;5;45m'; // Premium Deep Sky Blue
    }
    if (ctxLower.includes('auth') || ctxLower.includes('user') || ctxLower.includes('account')) {
      return '\x1b[38;5;78m'; // Premium Mint Green
    }
    if (ctxLower.includes('gateway') || ctxLower.includes('api') || ctxLower.includes('router') || ctxLower.includes('resolver')) {
      return '\x1b[38;5;39m'; // Premium Bright Blue
    }
    if (ctxLower.includes('nats') || ctxLower.includes('event') || ctxLower.includes('message')) {
      return '\x1b[38;5;214m'; // Premium Amber/Gold
    }
    if (ctxLower.includes('db') || ctxLower.includes('postgres') || ctxLower.includes('redis') || ctxLower.includes('repo')) {
      return '\x1b[38;5;203m'; // Premium Coral/Red-Orange
    }

    // Hash fallback for auto-assigning premium colors to other contexts
    const premiumColors = [
      '\x1b[38;5;75m',  // Light Blue
      '\x1b[38;5;78m',  // Mint
      '\x1b[38;5;176m', // Pastel Pink
      '\x1b[38;5;214m', // Gold
      '\x1b[38;5;147m', // Lavender
      '\x1b[38;5;209m', // Coral
      '\x1b[38;5;80m',  // Teal
      '\x1b[38;5;113m', // Lime Green
    ];

    let hash = 0;
    for (let i = 0; i < context.length; i++) {
      hash = context.charCodeAt(i) + ((hash << 5) - hash);
    }
    const index = Math.abs(hash) % premiumColors.length;
    return premiumColors[index];
  }

  private formatMessage(level: string, message: any, context?: string) {
    const store = LoggerContext.getStore() || {};
    const logObj = {
      timestamp: new Date().toISOString(),
      level,
      context: context || 'Application',
      message: typeof message === 'object' ? message : { text: message },
      ...store,
    };

    if (process.env.NODE_ENV === 'production') {
      return JSON.stringify(logObj);
    } else {
      const levelColor = this.getLevelColor(level);
      const ctxColor = this.getContextColor(logObj.context);

      const levelStr = `${levelColor}[${level.padEnd(7)}]\x1b[0m`;
      const timeStr = `\x1b[90m${logObj.timestamp}\x1b[0m`;
      const traceStr = logObj.traceId ? ` \x1b[38;5;198m[TraceID: ${logObj.traceId}]\x1b[0m` : '';
      const ctxStr = logObj.context ? ` ${ctxColor}[${logObj.context}]\x1b[0m` : '';
      const text = typeof message === 'object' ? JSON.stringify(message, null, 2) : message;

      return `${levelStr} ${timeStr}${traceStr}${ctxStr}: ${text}`;
    }
  }

  log(message: any, context?: string) {
    console.log(this.formatMessage('INFO', message, context));
  }

  error(message: any, trace?: string, context?: string) {
    console.error(this.formatMessage('ERROR', message, context || trace));
    if (trace && process.env.NODE_ENV !== 'production') {
      console.error(trace);
    }
  }

  warn(message: any, context?: string) {
    console.warn(this.formatMessage('WARN', message, context));
  }

  debug(message: any, context?: string) {
    console.log(this.formatMessage('DEBUG', message, context));
  }

  verbose(message: any, context?: string) {
    console.log(this.formatMessage('VERBOSE', message, context));
  }
}
