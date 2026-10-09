import { Injectable, LoggerService } from "@nestjs/common";
import { LoggerContext } from "./logger.context";

@Injectable()
export class StructuredLogger implements LoggerService {
  private isEnabled(level: "DEBUG" | "VERBOSE" | "INFO" | "WARN" | "ERROR") {
    const ranks: Record<string, number> = {
      debug: 0,
      verbose: 0,
      info: 1,
      warn: 2,
      warning: 2,
      error: 3,
    };
    const configured = (
      process.env.LOG_LEVEL ||
      (process.env.NODE_ENV === "production" ? "info" : "debug")
    )
      .toLowerCase()
      .trim();
    const minRank = ranks[configured] ?? ranks.info;
    const rank =
      level === "INFO" ? ranks.info : ranks[level.toLowerCase()] ?? ranks.info;
    return rank >= minRank;
  }

  private formatMessage(level: string, message: any, context?: string) {
    const store = LoggerContext.getStore() || {};
    const logObj = {
      timestamp: new Date().toISOString(),
      level,
      context: context || "Application",
      message: typeof message === "object" ? message : { text: message },
      ...store,
    };

    if (process.env.NODE_ENV === "production") {
      return JSON.stringify(logObj);
    } else {
      const traceInfo = logObj.traceId ? ` [TraceID: ${logObj.traceId}]` : "";
      const ctxInfo = logObj.context ? ` [${logObj.context}]` : "";
      const text =
        typeof message === "object"
          ? JSON.stringify(message, null, 2)
          : message;

      let color = "\x1b[0m"; // Reset
      if (level === "ERROR") color = "\x1b[31m"; // Red
      if (level === "WARN") color = "\x1b[33m"; // Yellow
      if (level === "DEBUG") color = "\x1b[35m"; // Magenta

      return `${color}[${level}] ${logObj.timestamp}${traceInfo}${ctxInfo}: ${text}\x1b[0m`;
    }
  }

  log(message: any, context?: string) {
    if (!this.isEnabled("INFO")) return;
    console.log(this.formatMessage("INFO", message, context));
  }

  error(message: any, trace?: string, context?: string) {
    if (!this.isEnabled("ERROR")) return;
    console.error(this.formatMessage("ERROR", message, context || trace));
    if (trace && process.env.NODE_ENV !== "production") {
      console.error(trace);
    }
  }

  warn(message: any, context?: string) {
    if (!this.isEnabled("WARN")) return;
    console.warn(this.formatMessage("WARN", message, context));
  }

  debug(message: any, context?: string) {
    if (!this.isEnabled("DEBUG")) return;
    console.log(this.formatMessage("DEBUG", message, context));
  }

  verbose(message: any, context?: string) {
    if (!this.isEnabled("VERBOSE")) return;
    console.log(this.formatMessage("VERBOSE", message, context));
  }
}
