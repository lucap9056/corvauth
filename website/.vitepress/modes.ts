export const deploymentModes = ["stateless", "managed", "external"] as const;

export type DeploymentMode = (typeof deploymentModes)[number];

export const modeStorageKey = "corvauth-mode";

export const modeQueryParam = "mode";

export function isDeploymentMode(value: unknown): value is DeploymentMode {
  return deploymentModes.includes(value as DeploymentMode);
}
