import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import { useGroupCreate } from './useGroupCreate';
import { PostcodeInput } from '../../components/PostcodeInput/PostcodeInput';
import { RadiusPicker } from '../../components/RadiusPicker/RadiusPicker';
import { AuthoritySelector } from '../../components/AuthoritySelector/AuthoritySelector';
import styles from './GroupCreatePage.module.css';

interface Props {
  repository: GroupsRepository;
  geocodingPort: GeocodingPort;
  authoritySearchPort: AuthoritySearchPort;
}

const STEP_LABELS: Record<string, string> = {
  postcode: 'Step 1 of 4',
  radius: 'Step 2 of 4',
  authority: 'Step 3 of 4',
  name: 'Step 4 of 4',
};

const STEP_TITLES: Record<string, string> = {
  postcode: 'Where is your group located?',
  radius: 'How far should alerts reach?',
  authority: 'Which local authority?',
  name: 'Name your group',
};

export function GroupCreatePage({ repository, geocodingPort, authoritySearchPort }: Props) {
  const navigate = useNavigate();
  const create = useGroupCreate(repository);
  const [groupName, setGroupName] = useState('');
  const [selectedRadius, setSelectedRadius] = useState(2000);

  async function handleSubmit() {
    const trimmed = groupName.trim();
    if (!trimmed) return;

    const result = await create.submit(trimmed);
    if (result) {
      navigate(`/groups/${result.groupId}`);
    }
  }

  return (
    <div className={styles.container}>
      <button className={styles.backButton} onClick={() => navigate('/groups')}>
        &larr; Back to Groups
      </button>

      <h1 className={styles.title}>Create Group</h1>

      <p className={styles.stepLabel}>{STEP_LABELS[create.step]}</p>
      <h2 className={styles.stepTitle}>{STEP_TITLES[create.step]}</h2>

      {create.step === 'postcode' && (
        <PostcodeInput
          geocodingPort={geocodingPort}
          onGeocode={create.setLocation}
        />
      )}

      {create.step === 'radius' && (
        <div>
          <RadiusPicker
            selectedMetres={selectedRadius}
            onSelect={(metres) => {
              setSelectedRadius(metres);
            }}
          />
          <button
            className={styles.submitButton}
            onClick={() => create.setRadius(selectedRadius)}
          >
            Continue
          </button>
        </div>
      )}

      {create.step === 'authority' && (
        <AuthoritySelector
          searchPort={authoritySearchPort}
          onSelect={create.setAuthority}
        />
      )}

      {create.step === 'name' && (
        <div>
          <input
            type="text"
            aria-label="Group name"
            className={styles.nameInput}
            value={groupName}
            onChange={(e) => setGroupName(e.target.value)}
            placeholder="e.g. Mill Road Residents"
          />
          <button
            className={styles.submitButton}
            disabled={!groupName.trim() || create.isSubmitting}
            onClick={handleSubmit}
          >
            {create.isSubmitting ? 'Creating...' : 'Create Group'}
          </button>
        </div>
      )}

      {create.error && (
        <p className={styles.error} role="alert">{create.error}</p>
      )}
    </div>
  );
}
