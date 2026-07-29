import { createStore } from "zustand/vanilla";
import { useStore } from "zustand";
import type { AppConfigResponse } from "../api/schemas";

interface ConfigState {
  cdnDomain: string;
  allowSignup: boolean;
  googleClientId: string;
  useSySso: boolean | null;
  authConfigError: string | null;
  daemonServerUrl: string;
  daemonAppUrl: string;
  // Self-host gate (#3433): when true, every "Create workspace" affordance
  // must be hidden. Defaults to false so unknown / older servers behave like
  // the managed-cloud case.
  workspaceCreationDisabled: boolean;
  setCdnDomain: (domain: string) => void;
  setAuthConfig: (config: {
    allowSignup: boolean;
    googleClientId?: string;
    useSySso: boolean;
    workspaceCreationDisabled?: boolean;
  }) => void;
  setDaemonConfig: (config: {
    daemonServerUrl?: string;
    daemonAppUrl?: string;
  }) => void;
  loadConfig: (request: () => Promise<AppConfigResponse>) => Promise<AppConfigResponse>;
}

export const configStore = createStore<ConfigState>((set) => ({
  cdnDomain: "",
  allowSignup: true,
  googleClientId: "",
  useSySso: null,
  authConfigError: null,
  daemonServerUrl: "",
  daemonAppUrl: "",
  workspaceCreationDisabled: false,
  setCdnDomain: (domain) => set({ cdnDomain: domain }),
  setAuthConfig: ({
    allowSignup,
    googleClientId = "",
    useSySso,
    workspaceCreationDisabled = false,
  }) => set({
    allowSignup,
    googleClientId,
    useSySso,
    authConfigError: null,
    workspaceCreationDisabled,
  }),
  setDaemonConfig: ({ daemonServerUrl = "", daemonAppUrl = "" }) =>
    set({ daemonServerUrl, daemonAppUrl }),
  loadConfig: async (request) => {
    set({ useSySso: null, authConfigError: null });
    try {
      const config = await request();
      set((state) => ({
        cdnDomain: config.cdn_domain || state.cdnDomain,
        allowSignup: config.allow_signup,
        googleClientId: config.google_client_id ?? "",
        useSySso: config.use_sy_sso,
        authConfigError: null,
        daemonServerUrl: config.daemon_server_url ?? "",
        daemonAppUrl: config.daemon_app_url ?? "",
        workspaceCreationDisabled: config.workspace_creation_disabled === true,
      }));
      return config;
    } catch (error) {
      set({
        useSySso: null,
        authConfigError: error instanceof Error ? error.message : "Failed to load app config",
      });
      throw error;
    }
  },
}));

export function useConfigStore(): ConfigState;
export function useConfigStore<T>(selector: (state: ConfigState) => T): T;
export function useConfigStore<T>(selector?: (state: ConfigState) => T) {
  return useStore(configStore, selector as (state: ConfigState) => T);
}
