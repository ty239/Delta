import {
  accounts,
  deposits,
  expectedTransfers,
  reconciliation,
  rulesets,
} from './mocks'
import type { ReconciliationStatus } from './types'

function formatCents(cents: number) {
  return (cents / 100).toLocaleString('en-US', {
    style: 'currency',
    currency: 'USD',
  })
}

function accountName(id: string) {
  return accounts.find((a) => a.id === id)?.name ?? id
}

const statusStyles: Record<ReconciliationStatus, string> = {
  matched: 'bg-accent-soft text-ink',
  partial: 'bg-surface-muted text-ink',
  missed: 'bg-data-red text-white',
  pending: 'border border-divider text-ink-muted',
}

const barColors = ['bg-data-blue', 'bg-accent', 'bg-data-red', 'bg-ink']

function App() {
  const ruleset = rulesets[0]
  const latest = deposits[deposits.length - 1]
  const transfers = expectedTransfers.filter((t) => t.depositId === latest.id)

  return (
    <main className="mx-auto max-w-[1200px] px-4 py-10 sm:px-8">
      <header className="mb-8">
        <h1 className="text-5xl font-extrabold tracking-tight text-white sm:text-7xl">
          Sluice
        </h1>
        <p className="mt-3 text-ink-muted">
          Percentage-based money allocation with reconciliation.
        </p>
      </header>

      <div className="grid gap-6 md:grid-cols-2">
        <section className="card">
          <h2 className="card-title">Accounts</h2>
          <ul className="mt-4 divide-y divide-divider">
            {accounts.map((a) => (
              <li key={a.id} className="flex items-center justify-between py-3">
                <div>
                  <p className="font-medium">{a.name}</p>
                  <p className="label">{a.institution}</p>
                </div>
                <p className={`money ${a.kind === 'credit' ? 'text-data-red' : ''}`}>
                  {a.kind === 'credit' ? '−' : ''}
                  {formatCents(a.balanceCents)}
                </p>
              </li>
            ))}
          </ul>
        </section>

        <section className="card">
          <h2 className="card-title">Your rules</h2>
          <p className="label mt-1">Every deposit is split like this</p>
          <div className="mt-5 flex h-3 overflow-hidden rounded-full">
            {ruleset.rules.map((r, i) => (
              <div
                key={r.id}
                className={barColors[i % barColors.length]}
                style={{ width: `${r.shareBps / 100}%` }}
              />
            ))}
          </div>
          <ul className="mt-5 grid grid-cols-2 gap-3">
            {ruleset.rules.map((r, i) => (
              <li key={r.id} className="flex items-center gap-2">
                <span
                  className={`size-2.5 rounded-full ${barColors[i % barColors.length]}`}
                />
                <span className="text-sm">{accountName(r.destinationAccountId)}</span>
                <span className="money ml-auto text-sm">{r.shareBps / 100}%</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="card md:col-span-2">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 className="card-title">Transfers from latest deposit</h2>
            <p className="label">
              {latest.description} ·{' '}
              <span className="money text-ink">{formatCents(latest.amountCents)}</span>
            </p>
          </div>
          <ul className="mt-4 divide-y divide-divider">
            {transfers.map((t) => {
              const status =
                reconciliation.find((r) => r.expectedTransferId === t.id)?.status ??
                'pending'
              return (
                <li key={t.id} className="flex items-center gap-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="font-medium">{accountName(t.toAccountId)}</p>
                    <p className="label">Due {t.dueDate}</p>
                  </div>
                  <span className={`pill ${statusStyles[status]}`}>{status}</span>
                  <p className="money w-24 text-right">{formatCents(t.amountCents)}</p>
                </li>
              )
            })}
          </ul>
        </section>
      </div>
    </main>
  )
}

export default App
