import { Link } from 'react-router-dom';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import type { InvitationId } from '../../domain/types';
import { useAcceptInvitation } from './useAcceptInvitation';
import styles from './AcceptInvitationPage.module.css';

interface Props {
  repository: GroupsRepository;
  invitationId: InvitationId;
}

export function AcceptInvitationPage({ repository, invitationId }: Props) {
  const { isLoading, isAccepted, error } = useAcceptInvitation(repository, invitationId);

  if (isLoading) {
    return (
      <div className={styles.container}>
        <p className={styles.loading}>Accepting invitation...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <span className={styles.icon} aria-hidden="true">&#10060;</span>
        <h1 className={styles.title}>Invitation Failed</h1>
        <p className={styles.message}>{error}</p>
        <Link to="/groups" className={styles.link}>
          Go to Groups
        </Link>
      </div>
    );
  }

  if (isAccepted) {
    return (
      <div className={styles.container}>
        <span className={styles.icon} aria-hidden="true">&#9989;</span>
        <h1 className={styles.title}>You're In!</h1>
        <p className={styles.message}>
          You've successfully joined the group.
        </p>
        <Link to="/groups" className={styles.link}>
          View Your Groups
        </Link>
      </div>
    );
  }

  return null;
}
