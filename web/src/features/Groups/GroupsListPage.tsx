import { Link } from 'react-router-dom';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import { useUserGroups } from './useUserGroups';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './GroupsListPage.module.css';

interface Props {
  repository: GroupsRepository;
  onCreateClick: () => void;
}

export function GroupsListPage({ repository, onCreateClick }: Props) {
  const { groups, isLoading, error } = useUserGroups(repository);

  if (isLoading) {
    return <div className={styles.loading}>Loading groups...</div>;
  }

  if (error) {
    return <div className={styles.error}>{error}</div>;
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Groups</h1>
        <button className={styles.createButton} onClick={onCreateClick}>
          + Create Group
        </button>
      </div>

      {groups.length === 0 ? (
        <EmptyState
          icon="👥"
          title="No groups yet"
          message="Create a group to share planning alerts with your neighbours and colleagues."
          actionLabel="Create Group"
          onAction={onCreateClick}
        />
      ) : (
        <ul className={styles.list}>
          {groups.map((group) => (
            <li key={group.groupId}>
              <Link to={`/groups/${group.groupId}`} className={styles.card}>
                <h2 className={styles.cardName}>{group.name}</h2>
                <div className={styles.cardMeta}>
                  <span className={styles.roleBadge}>{group.role}</span>
                  <span>{group.memberCount} {group.memberCount === 1 ? 'member' : 'members'}</span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
