import type { en } from "./en";

/** Plural forms per CLDR category; "other" is always required. */
export interface Plural {
  zero?: string;
  one?: string;
  two?: string;
  few?: string;
  many?: string;
  other: string;
}

export type MessageKey = keyof typeof en;

/** Same keys as English; a plural entry must stay plural. */
export type Dict = { [K in MessageKey]: (typeof en)[K] extends string ? string : Plural };

export type Params = Record<string, string | number>;
