import { StudioShell } from "./studio-shell";
import { PageIntro, Panel } from "./studio-primitives";

type PlaceholderPageProps = {
  eyebrow: string;
  title: string;
  description: string;
};

export function PlaceholderPage({ eyebrow, title, description }: PlaceholderPageProps) {
  return (
    <StudioShell>
      <div className="page-stack">
        <PageIntro eyebrow={eyebrow} title={title} description={description} />
        <Panel title="Placeholder" description={description}>
          <p className="muted-copy">{description}</p>
        </Panel>
      </div>
    </StudioShell>
  );
}
