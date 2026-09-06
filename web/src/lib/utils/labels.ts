/**
 * Returns suggested default runner labels based on the host operating system and architecture.
 * Defaults to "self-hosted,linux,arm64" if unspecified.
 */
export function getSuggestedRunnerLabels(hostOs?: string, hostArch?: string): string {
  const os = (hostOs && hostOs.trim() ? hostOs.trim() : "linux").toLowerCase();
  const arch = (hostArch && hostArch.trim() ? hostArch.trim() : "arm64").toLowerCase();
  return `self-hosted,${os},${arch}`;
}
