import { useState, useEffect } from "react";
import type { RuntimeInfo } from "@portier/shared";
import { fetchRuntimeInfo } from "../../api/portierApi.js";

export interface RuntimeInfoState {
  runtimeInfo: RuntimeInfo | null;
  runtimeLoading: boolean;
  runtimeUnavailable: boolean;
}

// useRuntimeInfo fetches the runtime/environment info once on mount, exposing
// loading and unavailable states. Behaviour matches the previous inline effect:
// a fetch rejection flips to "unavailable" rather than surfacing the error.
export function useRuntimeInfo(): RuntimeInfoState {
  const [runtimeInfo, setRuntimeInfo] = useState<RuntimeInfo | null>(null);
  const [runtimeLoading, setRuntimeLoading] = useState(true);
  const [runtimeUnavailable, setRuntimeUnavailable] = useState(false);

  useEffect(() => {
    fetchRuntimeInfo()
      .then((info) => {
        setRuntimeInfo(info);
        setRuntimeLoading(false);
      })
      .catch(() => {
        setRuntimeUnavailable(true);
        setRuntimeLoading(false);
      });
  }, []);

  return { runtimeInfo, runtimeLoading, runtimeUnavailable };
}
