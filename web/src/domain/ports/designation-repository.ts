import type { DesignationContext } from '../types';

export interface DesignationRepository {
  fetchDesignations(latitude: number, longitude: number): Promise<DesignationContext>;
}
