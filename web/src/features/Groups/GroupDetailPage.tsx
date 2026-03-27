import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import type { GroupId } from '../../domain/types';
import { useGroupDetail } from './useGroupDetail';
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog';
import styles from './GroupDetailPage.module.css';

interface Props {
  repository: GroupsRepository;
  groupId: GroupId;
  currentUserId: string;
}

export function GroupDetailPage({ repository, groupId, currentUserId }: Props) {
  const navigate = useNavigate();
  const detail = useGroupDetail(repository, groupId);
  const [inviteEmail, setInviteEmail] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);

  if (detail.isLoading) {
    return <div className={styles.loading}>Loading group...</div>;
  }

  if (detail.error || !detail.group) {
    return <div className={styles.error}>{detail.error ?? 'Group not found'}</div>;
  }

  const isOwner = detail.group.ownerId === currentUserId;

  async function handleInvite() {
    const trimmed = inviteEmail.trim();
    if (!trimmed) return;

    await detail.inviteMember(trimmed);
    setInviteEmail('');
  }

  async function handleDelete() {
    setConfirmDelete(false);
    await detail.deleteGroup();
    navigate('/groups');
  }

  async function handleRemoveMember(userId: string) {
    setConfirmRemove(null);
    await detail.removeMember(userId);
  }

  return (
    <div className={styles.container}>
      <Link to="/groups" className={styles.backLink}>
        &larr; Back to Groups
      </Link>

      <div className={styles.header}>
        <h1 className={styles.title}>{detail.group.name}</h1>
        {isOwner && (
          <button
            className={styles.deleteButton}
            onClick={() => setConfirmDelete(true)}
          >
            Delete Group
          </button>
        )}
      </div>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>
          Members ({detail.group.members.length})
        </h2>
        <ul className={styles.memberList}>
          {detail.group.members.map((member) => (
            <li key={member.userId} className={styles.memberRow}>
              <div className={styles.memberInfo}>
                <span className={styles.memberId}>{member.userId}</span>
                <span className={styles.roleBadge}>{member.role}</span>
              </div>
              {isOwner && member.userId !== currentUserId && (
                <button
                  className={styles.removeButton}
                  onClick={() => setConfirmRemove(member.userId)}
                >
                  Remove
                </button>
              )}
            </li>
          ))}
        </ul>
      </section>

      {isOwner && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Invite Member</h2>
          <div className={styles.inviteForm}>
            <input
              type="email"
              aria-label="Invitee email address"
              className={styles.inviteInput}
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              placeholder="Email address"
            />
            <button className={styles.inviteButton} onClick={handleInvite}>
              Send Invite
            </button>
          </div>
          {detail.actionError && (
            <p className={styles.actionError} role="alert">{detail.actionError}</p>
          )}
        </section>
      )}

      <ConfirmDialog
        open={confirmDelete}
        title="Delete Group"
        message={`Are you sure you want to delete "${detail.group.name}"? This cannot be undone.`}
        confirmLabel="Delete"
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />

      <ConfirmDialog
        open={confirmRemove !== null}
        title="Remove Member"
        message="Are you sure you want to remove this member from the group?"
        confirmLabel="Remove"
        onConfirm={() => confirmRemove && handleRemoveMember(confirmRemove)}
        onCancel={() => setConfirmRemove(null)}
      />
    </div>
  );
}
