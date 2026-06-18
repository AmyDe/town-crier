import styles from './Faq.module.css';

interface FaqItem {
  question: string;
  answer: string;
}

const FAQ_ITEMS: FaqItem[] = [
  {
    question: 'Where does the data come from?',
    answer:
      'Town Crier gets its data from PlanIt (planit.org.uk), which aggregates planning applications from local authorities across the UK. We poll those feeds regularly, so you see new applications shortly after they\u2019re published.',
  },
  {
    question: 'Which areas do you cover?',
    answer:
      'We cover local planning authorities across the whole UK: England, Scotland, Wales and Northern Ireland. If something looks missing for your area, let us know and we\u2019ll look into it.',
  },
  {
    question: 'Is there a free tier?',
    answer:
      'Yes. The Free plan monitors one watch zone and sends a weekly email digest, free forever. Upgrade any time for more zones, a wider radius, and instant alerts as applications land: push notifications on iOS and instant emails. The weekly digest stays available on every plan.',
  },
  {
    question: 'Can communities use Town Crier?',
    answer:
      'Yes. Neighbourhood forums, civic societies and residents\u2019 associations use Town Crier to keep track of planning activity in their area. The Pro plan adds more watch zones for wider coverage.',
  },
  {
    question: 'How quickly will I be notified?',
    answer:
      'Most applications appear within hours of being published by the local authority. Paid plans get instant push notifications. The Free plan sends a weekly digest email.',
  },
];

export function Faq() {
  return (
    <section id="faq" className={styles.section}>
      <h2 className={styles.heading}>Frequently Asked Questions</h2>
      <div className={styles.list}>
        {FAQ_ITEMS.map((item) => (
          <details key={item.question} className={styles.details}>
            <summary className={styles.summary}>{item.question}</summary>
            <p className={styles.answer}>{item.answer}</p>
          </details>
        ))}
      </div>
    </section>
  );
}
