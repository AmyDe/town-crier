import { useEffect, type ReactNode } from 'react';
import { Link } from 'react-router';
import {
  DISCORD_EXAMPLE,
  HEADERS_EXAMPLE,
  NODE_EXAMPLE,
  PAYLOAD_EXAMPLE,
  PYTHON_EXAMPLE,
  SLACK_EXAMPLE,
} from './examples';
import styles from './WebhookDocsPage.module.css';

const PAGE_TITLE = 'Webhooks | Town Crier';

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionHeading}>{title}</h2>
      {children}
    </section>
  );
}

// Focusable so that keyboard users can scroll long lines.
function CodeBlock({ label, code }: { label: string; code: string }) {
  return (
    <pre className={styles.codeBlock} tabIndex={0} aria-label={label}>
      <code>{code}</code>
    </pre>
  );
}

export function WebhookDocsPage() {
  useEffect(() => {
    const previousTitle = document.title;
    document.title = PAGE_TITLE;
    return () => {
      document.title = previousTitle;
    };
  }, []);

  return (
    <main className={styles.container}>
      <Link to="/" className={styles.home}>Town Crier</Link>
      <h1 className={styles.title}>Webhooks</h1>
      <p className={styles.intro}>
        A webhook sends your Town Crier alerts to a URL you choose, as signed JSON, a Slack message
        or a Discord message. Use it to feed a CRM, a spreadsheet, a team channel or your own code.
        Webhooks are part of Pro, and each account can have one.
      </p>

      <Section title="Set up a webhook">
        <p>
          Sign in on the web, open <Link to="/settings">Settings</Link>, enter your URL, choose
          JSON, Slack or Discord and save. We show your signing secret once, so copy it then. If
          you lose it, rotate it to get a new one. The old one stops working.
        </p>
        <p>
          Send test event sends a made-up application to your URL straight away, with the type{' '}
          <code>test</code>. We don&apos;t retry it, and it doesn&apos;t count towards switching
          your webhook off. You can send one every 10 seconds, even when the webhook is switched
          off.
        </p>
      </Section>

      <Section title="What sends a request">
        <ul>
          <li>A new planning application in one of your watch zones.</li>
          <li>A decision on an application in a watch zone, or on one you&apos;ve saved.</li>
        </ul>
        <p>
          Your push notification settings don&apos;t affect webhooks. Paused watch zones and
          keyword filters do, because they stop the alert being created. We don&apos;t queue
          anything while your webhook is switched off or your account isn&apos;t on Pro.
        </p>
      </Section>

      <Section title="When requests are sent">
        <p>
          We check for new planning data every hour. Requests go out at the end of that check,
          after push notifications. If we hold an alert back, for example overnight, the request
          waits too.
        </p>
      </Section>

      <Section title="Which addresses work">
        <ul>
          <li>It must start with <code>https://</code> and use port 443.</li>
          <li>It must resolve to a public IP address.</li>
          <li>It can&apos;t contain a username or password, or be longer than 2048 bytes.</li>
          <li>
            Names such as <code>localhost</code>, and names ending in <code>.localhost</code>,{' '}
            <code>.local</code> or <code>.internal</code>, are refused.
          </li>
          <li>We don&apos;t follow redirects.</li>
        </ul>
      </Section>

      <Section title="JSON payload">
        <p>
          With the JSON format, each request carries one alert. This is{' '}
          <code>schemaVersion</code> 1.
        </p>
        <CodeBlock label="Example JSON payload" code={PAYLOAD_EXAMPLE} />
        <ul>
          <li>
            <code>eventId</code> identifies the alert. It stays the same when we retry.
          </li>
          <li>
            <code>type</code> is <code>application.new</code>,{' '}
            <code>application.decision</code> or <code>test</code>.
          </li>
          <li>
            <code>occurredAt</code> is when we created the alert, in UTC (RFC 3339).
          </li>
          <li>
            <code>watchZone</code> is <code>null</code> for a decision on a saved application that
            isn&apos;t in one of your zones, and for a test.
          </li>
          <li>
            <code>sources</code> says why you got the alert: <code>zone</code>,{' '}
            <code>saved</code> or both. It&apos;s empty for a test.
          </li>
          <li>
            <code>reference</code>, <code>authorityName</code>, <code>address</code> and{' '}
            <code>description</code> are always strings. Any other field in{' '}
            <code>application</code> can be <code>null</code>, but it&apos;s never left out.
          </li>
          <li>
            Dates look like <code>YYYY-MM-DD</code>.
          </li>
          <li>
            <code>councilUrl</code> is the council&apos;s record. <code>shareUrl</code> is the Town
            Crier page for the application.
          </li>
        </ul>
      </Section>

      <Section title="Request headers">
        <p>Every request is a POST with these headers, in every format.</p>
        <CodeBlock label="Request headers" code={HEADERS_EXAMPLE} />
        <p>
          Header names aren&apos;t case sensitive, and you might see them written differently, such
          as <code>X-Towncrier-Event-Id</code>.
        </p>
      </Section>

      <Section title="Check the signature">
        <p>
          Check the signature before you trust a request. It&apos;s <code>v1=</code> followed by
          the lowercase hex HMAC-SHA256 of <code>{'<timestamp>.<raw body>'}</code>. The timestamp
          is the value of <code>X-TownCrier-Timestamp</code>. The key is your whole signing secret,
          including <code>tcwh_</code>.
        </p>
        <ul>
          <li>Use the raw body, exactly as it arrived. Parsed and re-encoded JSON won&apos;t match.</li>
          <li>Compare the two signatures with a constant-time function.</li>
          <li>Reject any timestamp older than 5 minutes.</li>
        </ul>
        <h3 className={styles.subHeading}>Node.js</h3>
        <CodeBlock label="Node.js signature check" code={NODE_EXAMPLE} />
        <h3 className={styles.subHeading}>Python</h3>
        <CodeBlock label="Python signature check" code={PYTHON_EXAMPLE} />
      </Section>

      <Section title="Retries and switching off">
        <p>
          Reply with any 2xx status within 5 seconds. Anything else counts as a failure, including
          a redirect or a timeout. Reply first and do any slow work afterwards.
        </p>
        <ul>
          <li>
            With JSON we send alerts one at a time, oldest first, up to 100 in each hourly run. At
            the first failure we stop, and try again on the next run.
          </li>
          <li>
            With Slack or Discord, one failed message counts as a failed attempt for every alert in
            it.
          </li>
          <li>Each alert gets up to 6 attempts. After that we give up on it.</li>
          <li>We delete delivery records after 7 days, whether they were sent or not.</li>
          <li>
            We switch your webhook off after 24 failed runs in a row, which is about a day. A run
            fails when nothing gets through. Any successful delivery resets the count.
          </li>
          <li>
            If your URL answers <code>410 Gone</code>, we switch it off at once.
          </li>
        </ul>
        <p>
          When we switch a webhook off, we drop the alerts still waiting for it and don&apos;t send
          them later. We email you if we have your email address. Your alerts still arrive in the
          app. To switch the webhook back on, check the address and save it again in Settings.
        </p>
      </Section>

      <Section title="Duplicates">
        <p>
          Delivery is at least once, so now and then the same alert can arrive twice. With JSON,
          keep the <code>X-TownCrier-Event-Id</code> values you&apos;ve handled and ignore repeats.
          A retry has the same body and event ID, with a new timestamp and signature.
        </p>
      </Section>

      <Section title="Slack format">
        <p>
          Choose Slack and paste in a Slack incoming webhook URL. You get one summary message for
          each hourly run, not one message for each alert. The body is{' '}
          <code>{'{ "text": "..." }'}</code>, and the text looks like this.
        </p>
        <CodeBlock label="Example Slack message" code={SLACK_EXAMPLE} />
        <ul>
          <li>A message lists up to 10 alerts, then says &quot;…and N more in the Town Crier app&quot;.</li>
          <li>Descriptions are cut off after 150 characters.</li>
          <li>The reference links to the Town Crier page when there is one.</li>
          <li>Slack ignores the Town Crier headers, so you don&apos;t need to check the signature.</li>
        </ul>
      </Section>

      <Section title="Discord format">
        <p>
          Choose Discord and paste in the webhook URL from your Discord channel&apos;s Integrations
          settings, just as it is. Don&apos;t add <code>/slack</code> to the end. You get one summary
          message for each hourly run, not one message for each alert. The message&apos;s{' '}
          <code>content</code> looks like this.
        </p>
        <CodeBlock label="Example Discord message" code={DISCORD_EXAMPLE} />
        <ul>
          <li>A message lists up to 10 alerts, then says &quot;…and N more in the Town Crier app&quot;.</li>
          <li>It lists fewer if they won&apos;t fit in Discord&apos;s limit of 2,000 characters.</li>
          <li>Descriptions are cut off after 150 characters.</li>
          <li>The reference links to the Town Crier page when there is one.</li>
          <li>Discord ignores the Town Crier headers, so there&apos;s no signature to check.</li>
          <li>Mentions in council text never ping anyone.</li>
        </ul>
      </Section>
    </main>
  );
}
