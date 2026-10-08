// API contract between the frontend and the backend.
//
// Money is always integer cents. Percentages are integer basis points
// (10000 = 100%). Dates are ISO 8601 strings: dates as YYYY-MM-DD,
// timestamps with a time and offset.

export type Cents = number
export type BasisPoints = number
export type ISODate = string
export type ISODateTime = string

export type AccountKind = 'checking' | 'savings' | 'retirement' | 'credit'

export interface Account {
  id: string
  name: string
  institution: string
  kind: AccountKind
  /** For credit accounts, the amount owed (positive). */
  balanceCents: Cents
  /** Annual yield on savings, or APR on credit. */
  rateBps?: BasisPoints
  /** Yearly contribution cap, e.g. the Roth IRA limit. */
  annualContributionLimitCents?: Cents
  ytdContributionsCents?: Cents
}

export interface AllocationRule {
  id: string
  destinationAccountId: string
  shareBps: BasisPoints
}

/**
 * A versioned set of rules. Editing rules creates a new version, so
 * expected transfers always point at the version that produced them.
 * The shares of a ruleset's rules sum to exactly 10000.
 */
export interface Ruleset {
  id: string
  version: number
  effectiveFrom: ISODateTime
  rules: AllocationRule[]
}

export interface Deposit {
  id: string
  accountId: string
  amountCents: Cents
  postedAt: ISODateTime
  description: string
}

/** What Sluice says should be moved after a deposit lands. */
export interface ExpectedTransfer {
  id: string
  depositId: string
  rulesetId: string
  rulesetVersion: number
  ruleId: string
  fromAccountId: string
  toAccountId: string
  amountCents: Cents
  dueDate: ISODate
}

/** A transaction actually observed on an account. */
export interface Transaction {
  id: string
  accountId: string
  /** Positive is money in, negative is money out. */
  amountCents: Cents
  postedAt: ISODateTime
  description: string
}

export type ReconciliationStatus =
  | 'matched' // found, amount matches
  | 'partial' // found, but the amount is off
  | 'missed' // past due, nothing found
  | 'pending' // not yet due, nothing found

export interface ReconciliationEntry {
  expectedTransferId: string
  status: ReconciliationStatus
  matchedTransactionId?: string
  /** Actual minus expected. Zero when matched; negative when short. */
  differenceCents: Cents
}
