import { FAQ_ITEMS } from './faqItems';
import styles from './Faq.module.css';

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
