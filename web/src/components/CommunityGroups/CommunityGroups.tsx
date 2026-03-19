import styles from './CommunityGroups.module.css';

interface Feature {
  icon: string;
  title: string;
  description: string;
}

const FEATURES: Feature[] = [
  {
    icon: '👥',
    title: 'Create a group',
    description:
      'Set up a community watch group for your street, estate, or neighbourhood. Pool your coverage and never miss a nearby application.',
  },
  {
    icon: '🏘️',
    title: 'Invite neighbours',
    description:
      'Share a simple invite link with neighbours, friends, or your residents\u2019 association. Everyone stays in the loop automatically.',
  },
  {
    icon: '📋',
    title: 'Coordinate responses',
    description:
      'Discuss applications within your group and coordinate formal responses to the council. A united voice carries more weight.',
  },
];

export function CommunityGroups() {
  return (
    <section id="community-groups" className={styles.section}>
      <h2 className={styles.heading}>Stronger together</h2>
      <p className={styles.subheading}>
        Planning decisions affect whole streets, not just individual homes. Form a community group to watch your area together and coordinate responses.
      </p>
      <ul className={styles.grid}>
        {FEATURES.map((feature) => (
          <li key={feature.title} className={styles.card}>
            <span className={styles.icon} aria-hidden="true">
              {feature.icon}
            </span>
            <h3 className={styles.featureTitle}>{feature.title}</h3>
            <p className={styles.featureDescription}>{feature.description}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
