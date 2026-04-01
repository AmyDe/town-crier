import styles from './Faq.module.css';

interface FaqItem {
  question: string;
  answer: string;
}

const FAQ_ITEMS: FaqItem[] = [
  {
    question: 'Where does the data come from?',
    answer:
      'Town Crier sources its data from PlanIt (planit.org.uk), the UK\u2019s most comprehensive aggregator of planning application data. We poll local authority feeds regularly so you see new applications shortly after they\u2019re published.',
  },
  {
    question: 'Which areas do you cover?',
    answer:
      'We cover local planning authorities across England, Scotland, and Wales. Coverage is expanding \u2014 if your area isn\u2019t listed yet, let us know and we\u2019ll prioritise it.',
  },
  {
    question: 'Is there a free tier?',
    answer:
      'Yes. The Free plan lets you monitor one watch zone with daily email digests at no cost, forever. Upgrade any time if you need more zones, wider radii, or instant push notifications.',
  },
  {
    question: 'Can communities use Town Crier?',
    answer:
      'Absolutely. Neighbourhood forums, civic societies, and residents\u2019 associations use Town Crier to stay on top of planning activity in their area. The Pro plan supports multiple watch zones for broader coverage.',
  },
  {
    question: 'How quickly will I be notified?',
    answer:
      'Most applications appear within hours of being published by the local authority. Paid plans receive instant push notifications; the Free plan delivers a daily digest email each morning.',
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
