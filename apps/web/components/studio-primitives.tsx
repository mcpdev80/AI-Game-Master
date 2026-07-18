import type { ReactNode } from "react";

export function PageIntro({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow: string;
  title: string;
  description: string;
  actions?: ReactNode;
}) {
  return (
    <header className="page-intro">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1 className="page-title">{title}</h1>
        <p className="page-description">{description}</p>
      </div>
      {actions ? <div className="page-actions">{actions}</div> : null}
    </header>
  );
}

export function Panel({
  title,
  description,
  action,
  children,
  className = "",
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`studio-panel ${className}`.trim()}>
      <div className="studio-panel__header">
        <div>
          <h2 className="studio-panel__title">{title}</h2>
          {description ? <p className="studio-panel__description">{description}</p> : null}
        </div>
        {action ? <div className="studio-panel__action">{action}</div> : null}
      </div>
      {children}
    </section>
  );
}

export function StatusPill({
  children,
  tone = "default",
}: {
  children: ReactNode;
  tone?: "default" | "ready" | "warning" | "live" | "info";
}) {
  return <span className={`status-pill status-pill--${tone}`}>{children}</span>;
}

export function StatCard({
  label,
  value,
  detail,
}: {
  label: string;
  value: string | number;
  detail?: string;
}) {
  return (
    <article className="stat-card">
      <p className="stat-card__label">{label}</p>
      <p className="stat-card__value">{value}</p>
      {detail ? <p className="stat-card__detail">{detail}</p> : null}
    </article>
  );
}
