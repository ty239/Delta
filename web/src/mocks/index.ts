// Fake data for Phase 1. Shapes follow src/types; the backend will serve
// the same shapes later.
//
// The expected transfers below were computed with largest-remainder
// rounding, so each deposit's splits sum exactly to the deposit.
// "Today" in this data is 2026-10-07.

import type {
  Account,
  Deposit,
  ExpectedTransfer,
  ReconciliationEntry,
  Ruleset,
  Transaction,
} from '../types'

export const accounts: Account[] = [
  {
    id: 'acc_checking',
    name: 'Everyday Checking',
    institution: 'First Plains Bank',
    kind: 'checking',
    balanceCents: 84_212,
  },
  {
    id: 'acc_savings',
    name: 'High-Yield Savings',
    institution: 'Northwind Savings',
    kind: 'savings',
    balanceCents: 312_450,
    rateBps: 400,
  },
  {
    id: 'acc_roth',
    name: 'Roth IRA',
    institution: 'Contoso Brokerage',
    kind: 'retirement',
    balanceCents: 528_900,
    annualContributionLimitCents: 750_000,
    ytdContributionsCents: 410_000,
  },
  {
    id: 'acc_card',
    name: 'Rewards Card',
    institution: 'First Plains Bank',
    kind: 'credit',
    balanceCents: 146_075,
    rateBps: 2499,
  },
]

export const rulesets: Ruleset[] = [
  {
    id: 'rs_main',
    version: 1,
    effectiveFrom: '2026-09-01T00:00:00-05:00',
    rules: [
      { id: 'rule_savings', destinationAccountId: 'acc_savings', shareBps: 2500 },
      { id: 'rule_roth', destinationAccountId: 'acc_roth', shareBps: 3300 },
      { id: 'rule_card', destinationAccountId: 'acc_card', shareBps: 1000 },
      // Stays in checking for spending, so it produces no transfer.
      { id: 'rule_spending', destinationAccountId: 'acc_checking', shareBps: 3200 },
    ],
  },
]

export const deposits: Deposit[] = [
  {
    id: 'dep_1',
    accountId: 'acc_checking',
    amountCents: 30_000,
    postedAt: '2026-09-11T09:00:00-05:00',
    description: 'PAYROLL ACME CO',
  },
  {
    id: 'dep_2',
    accountId: 'acc_checking',
    amountCents: 41_237,
    postedAt: '2026-09-25T09:00:00-05:00',
    description: 'PAYROLL ACME CO',
  },
  {
    id: 'dep_3',
    accountId: 'acc_checking',
    amountCents: 28_750,
    postedAt: '2026-10-06T09:00:00-05:00',
    description: 'PAYROLL ACME CO',
  },
]

function transfer(
  depositId: string,
  ruleId: string,
  toAccountId: string,
  amountCents: number,
  dueDate: string,
): ExpectedTransfer {
  return {
    id: `xfer_${depositId}_${ruleId}`,
    depositId,
    rulesetId: 'rs_main',
    rulesetVersion: 1,
    ruleId,
    fromAccountId: 'acc_checking',
    toAccountId,
    amountCents,
    dueDate,
  }
}

export const expectedTransfers: ExpectedTransfer[] = [
  // dep_1: $300.00 -> 75.00 / 99.00 / 30.00 (96.00 stays)
  transfer('dep_1', 'rule_savings', 'acc_savings', 7_500, '2026-09-14'),
  transfer('dep_1', 'rule_roth', 'acc_roth', 9_900, '2026-09-14'),
  transfer('dep_1', 'rule_card', 'acc_card', 3_000, '2026-09-14'),
  // dep_2: $412.37 -> 103.09 / 136.08 / 41.24 (131.96 stays)
  transfer('dep_2', 'rule_savings', 'acc_savings', 10_309, '2026-09-28'),
  transfer('dep_2', 'rule_roth', 'acc_roth', 13_608, '2026-09-28'),
  transfer('dep_2', 'rule_card', 'acc_card', 4_124, '2026-09-28'),
  // dep_3: $287.50 -> 71.88 / 94.87 / 28.75 (92.00 stays)
  transfer('dep_3', 'rule_savings', 'acc_savings', 7_188, '2026-10-09'),
  transfer('dep_3', 'rule_roth', 'acc_roth', 9_487, '2026-10-09'),
  transfer('dep_3', 'rule_card', 'acc_card', 2_875, '2026-10-09'),
]

export const transactions: Transaction[] = [
  // Deposits landing in checking
  { id: 'txn_101', accountId: 'acc_checking', amountCents: 30_000, postedAt: '2026-09-11T09:00:00-05:00', description: 'PAYROLL ACME CO' },
  { id: 'txn_102', accountId: 'acc_checking', amountCents: 41_237, postedAt: '2026-09-25T09:00:00-05:00', description: 'PAYROLL ACME CO' },
  { id: 'txn_103', accountId: 'acc_checking', amountCents: 28_750, postedAt: '2026-10-06T09:00:00-05:00', description: 'PAYROLL ACME CO' },

  // dep_1: all three transfers made
  { id: 'txn_201', accountId: 'acc_savings', amountCents: 7_500, postedAt: '2026-09-12T14:10:00-05:00', description: 'TRANSFER FROM FIRST PLAINS' },
  { id: 'txn_202', accountId: 'acc_roth', amountCents: 9_900, postedAt: '2026-09-13T10:02:00-05:00', description: 'ROTH CONTRIBUTION' },
  { id: 'txn_203', accountId: 'acc_card', amountCents: 3_000, postedAt: '2026-09-12T08:45:00-05:00', description: 'PAYMENT THANK YOU' },

  // dep_2: savings made, Roth skipped, card paid $1 short
  { id: 'txn_204', accountId: 'acc_savings', amountCents: 10_309, postedAt: '2026-09-26T11:30:00-05:00', description: 'TRANSFER FROM FIRST PLAINS' },
  { id: 'txn_205', accountId: 'acc_card', amountCents: 4_024, postedAt: '2026-09-27T19:12:00-05:00', description: 'PAYMENT THANK YOU' },

  // dep_3: savings done early, the rest not yet due
  { id: 'txn_206', accountId: 'acc_savings', amountCents: 7_188, postedAt: '2026-10-06T16:40:00-05:00', description: 'TRANSFER FROM FIRST PLAINS' },
]

export const reconciliation: ReconciliationEntry[] = [
  { expectedTransferId: 'xfer_dep_1_rule_savings', status: 'matched', matchedTransactionId: 'txn_201', differenceCents: 0 },
  { expectedTransferId: 'xfer_dep_1_rule_roth', status: 'matched', matchedTransactionId: 'txn_202', differenceCents: 0 },
  { expectedTransferId: 'xfer_dep_1_rule_card', status: 'matched', matchedTransactionId: 'txn_203', differenceCents: 0 },
  { expectedTransferId: 'xfer_dep_2_rule_savings', status: 'matched', matchedTransactionId: 'txn_204', differenceCents: 0 },
  { expectedTransferId: 'xfer_dep_2_rule_roth', status: 'missed', differenceCents: -13_608 },
  { expectedTransferId: 'xfer_dep_2_rule_card', status: 'partial', matchedTransactionId: 'txn_205', differenceCents: -100 },
  { expectedTransferId: 'xfer_dep_3_rule_savings', status: 'matched', matchedTransactionId: 'txn_206', differenceCents: 0 },
  { expectedTransferId: 'xfer_dep_3_rule_roth', status: 'pending', differenceCents: 0 },
  { expectedTransferId: 'xfer_dep_3_rule_card', status: 'pending', differenceCents: 0 },
]
