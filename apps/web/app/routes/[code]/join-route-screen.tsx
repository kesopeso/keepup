"use client";

import { SubmitEvent, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  clearRouteAuth,
  getProfile,
  getRouteAuth,
  saveProfile,
  saveRouteAuth,
  type TransportMode,
} from "../../../lib/identity-storage";
import {
  connectRouteLiveSocket,
  type LiveSocket,
  type RouteLiveSocketConnection,
} from "../../../lib/live-connection";
import {
  applyMemberLiveEvent,
  isMemberLiveEvent,
  parseLiveEvent,
} from "../../../lib/live-route-state";
import {
  appendLiveRoutePoint,
  mergeSnapshotIntoMapState,
  routeSnapshotToMapState,
  updateMapMemberStatus,
} from "../../../lib/map/snapshot-map-state";
import {
  navigationService,
  type NavigationPosition,
} from "../../../lib/navigation-service";
import {
  ApiError,
  closeRoute,
  deleteRoute,
  getRouteAccess,
  getRouteSnapshot,
  joinRoute,
  routeWebSocketUrl,
  type RouteAccess,
  type RouteSnapshot,
  type RouteSummary,
  type SnapshotMember,
} from "../../../lib/routes-api";
import { RouteMap } from "../../components/route-map";
import {
  Brand,
  Modal,
  NotificationStack,
  StatusBadge,
  TransportField,
  type NotificationItem,
  transportLabels,
} from "../../components/ui";

export function JoinRouteScreen({ code }: { code: string }) {
  const router = useRouter();
  const [access, setAccess] = useState<RouteAccess | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [transportMode, setTransportMode] = useState<TransportMode>("car");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isJoining, setIsJoining] = useState(false);
  const [memberToken, setMemberToken] = useState("");
  const [ownerToken, setOwnerToken] = useState("");
  const [snapshot, setSnapshot] = useState<RouteSnapshot | null>(null);

  useEffect(() => {
    let isMounted = true;

    const profile = getProfile();
    setDisplayName(profile.displayName);
    setTransportMode(profile.transportMode);

    const routeAuth = getRouteAuth(code);
    setOwnerToken(routeAuth?.ownerToken ?? "");
    if (routeAuth?.memberToken) {
      setMemberToken(routeAuth.memberToken);
      getRouteSnapshot(code, routeAuth.memberToken)
        .then((routeSnapshot) => {
          if (!isMounted) {
            return;
          }

          setSnapshot(routeSnapshot);
          setIsLoading(false);
        })
        .catch((caughtError) => {
          if (!isMounted) {
            return;
          }

          if (
            caughtError instanceof ApiError &&
            caughtError.code === "unauthorized"
          ) {
            clearRouteAuth(code);
            setMemberToken("");
            loadRouteAccess(
              code,
              () => isMounted,
              setAccess,
              setError,
              setIsLoading,
            );
            return;
          }

          setError(
            caughtError instanceof Error
              ? caughtError.message
              : "Could not load this route.",
          );
          setIsLoading(false);
        });
      return;
    }

    loadRouteAccess(code, () => isMounted, setAccess, setError, setIsLoading);

    return () => {
      isMounted = false;
    };
  }, [code]);

  const canJoin = useMemo(
    () =>
      displayName.trim() !== "" &&
      (!access?.requiresPassword || password !== "") &&
      !isJoining,
    [access?.requiresPassword, displayName, isJoining, password],
  );

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");

    if (!canJoin) {
      setError("Fill in the required details to join.");
      return;
    }

    setIsJoining(true);

    try {
      const profile = saveProfile({
        displayName,
        transportMode,
      });

      const result = await joinRoute(code, {
        clientId: profile.clientId,
        displayName: profile.displayName,
        transportMode: profile.transportMode,
        password,
      });

      saveRouteAuth(result.route.code, {
        memberToken: result.memberToken,
      });
      setMemberToken(result.memberToken);
      const routeSnapshot = await getRouteSnapshot(
        result.route.code,
        result.memberToken,
      );
      setSnapshot(routeSnapshot);
    } catch (caughtError) {
      setError(
        caughtError instanceof Error
          ? caughtError.message
          : "Could not join this route.",
      );
    } finally {
      setIsJoining(false);
    }
  }

  if (isLoading) {
    return (
      <section className="route-shell">
        <RouteHeader code={code} label="Route" />
        <p className="route-status loading-status" role="status">
          Loading your route...
        </p>
      </section>
    );
  }

  if (snapshot) {
    return (
      <RouteSnapshotShell
        code={code}
        memberToken={memberToken}
        ownerToken={ownerToken}
        onDeleted={() => router.replace("/")}
        onSnapshotChange={setSnapshot}
        snapshot={snapshot}
      />
    );
  }

  if (!access) {
    return (
      <section className="route-shell">
        <RouteHeader code={code} label="Route" />
        <div className="route-panel">
          <p className="form-error" role="alert">
            {error || "Could not load this route."}
          </p>
        </div>
      </section>
    );
  }

  return (
    <form className="route-form" onSubmit={handleSubmit} aria-busy={isJoining}>
      <RouteHeader code={access.code} label="Join route" title={access.name} />

      {access.description ? (
        <p className="route-description">{access.description}</p>
      ) : null}

      <div className="route-meta">
        <span>{access.status === "closed" ? "Closed archive" : "Active"}</span>
        <span>
          {access.sharingPolicy === "everyone_can_share"
            ? "Everyone can share"
            : "Joiners view only"}
        </span>
        {access.requiresPassword ? <span>Password required</span> : null}
      </div>

      <div className="field-grid">
        <label className="field">
          <span>Your name</span>
          <input
            autoComplete="name"
            name="displayName"
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Ana"
            required
            value={displayName}
          />
        </label>

        <TransportField value={transportMode} onChange={setTransportMode} />
      </div>

      {access.requiresPassword ? (
        <label className="field">
          <span>Password</span>
          <input
            autoComplete="current-password"
            name="password"
            onChange={(event) => setPassword(event.target.value)}
            required
            type="password"
            value={password}
          />
        </label>
      ) : null}

      {error ? (
        <p className="form-error" role="alert">
          {error}
        </p>
      ) : null}

      <button className="primary-action" disabled={!canJoin} type="submit">
        {isJoining
          ? "Joining..."
          : access.status === "closed"
            ? "View archive"
            : "Join route"}
      </button>
      <p className="privacy-note">
        Your location stays private until you start sharing.
      </p>
    </form>
  );
}

function loadRouteAccess(
  code: string,
  shouldUpdate: () => boolean,
  setAccess: (access: RouteAccess) => void,
  setError: (error: string) => void,
  setIsLoading: (isLoading: boolean) => void,
) {
  getRouteAccess(code)
    .then((routeAccess) => {
      if (!shouldUpdate()) {
        return;
      }

      setAccess(routeAccess);
      setIsLoading(false);
    })
    .catch((caughtError) => {
      if (!shouldUpdate()) {
        return;
      }

      setError(
        caughtError instanceof Error
          ? caughtError.message
          : "Could not load this route.",
      );
      setIsLoading(false);
    });
}

function RouteSnapshotShell({
  code,
  memberToken,
  ownerToken,
  onDeleted,
  onSnapshotChange,
  snapshot,
}: {
  code: string;
  memberToken: string;
  ownerToken: string;
  onDeleted: () => void;
  onSnapshotChange: (snapshot: RouteSnapshot) => void;
  snapshot: RouteSnapshot;
}) {
  const [sharingAction, setSharingAction] = useState<"start" | "stop" | null>(
    null,
  );
  const [membersExpanded, setMembersExpanded] = useState(true);
  const [showSettings, setShowSettings] = useState(false);
  const [shareNotices, setShareNotices] = useState<NotificationItem[]>([]);
  const nextShareNoticeId = useRef(0);
  const shareNoticeTimers = useRef(new Map<number, number[]>());
  const [manualShareUrl, setManualShareUrl] = useState("");
  const [isSharingLink, setIsSharingLink] = useState(false);
  const [sharingError, setSharingError] = useState("");
  const [liveTrackingError, setLiveTrackingError] = useState("");
  const [liveConnectionRejected, setLiveConnectionRejected] = useState(false);
  const [liveConnectionReady, setLiveConnectionReady] = useState(false);
  const [showStaleRecovery, setShowStaleRecovery] = useState(
    () =>
      snapshot.route.status === "active" && snapshot.viewer.status === "stale",
  );
  const [lifecycleAction, setLifecycleAction] = useState<
    "close" | "delete" | null
  >(null);
  const [deleteConfirmation, setDeleteConfirmation] = useState("");
  const [lifecycleError, setLifecycleError] = useState("");
  const [isLifecycleSubmitting, setIsLifecycleSubmitting] = useState(false);
  const websocketRef = useRef<LiveSocket | null>(null);
  const snapshotRef = useRef(snapshot);
  const [mapState, setMapState] = useState(() =>
    routeSnapshotToMapState(snapshot),
  );
  const sortedMembers = [...snapshot.members].sort(compareMembers);
  const canUseSharingControl =
    memberToken !== "" &&
    snapshot.route.status === "active" &&
    liveConnectionReady &&
    !sharingAction &&
    (snapshot.viewer.canStartSharing || snapshot.viewer.canStopSharing);
  const sharingControlLabel = snapshot.viewer.canStopSharing
    ? "Stop sharing"
    : "Start sharing location";
  const sharingControlBusyLabel =
    sharingAction === "stop" ? "Stopping..." : "Starting...";
  const isViewerTracking =
    memberToken !== "" &&
    snapshot.route.status === "active" &&
    snapshot.viewer.status === "tracking";

  const isArchive = snapshot.route.status === "closed";
  const sharingCount = snapshot.members.filter(
    (member) => member.status === "tracking",
  ).length;
  const occupiedSlots = snapshot.members.filter(
    (member) => member.status === "tracking" || member.status === "stale",
  ).length;
  const sharingTitle = isArchive
    ? "Route archive"
    : !liveConnectionReady
      ? "Live connection unavailable"
      : snapshot.viewer.status === "tracking"
        ? "You're sharing location"
        : snapshot.viewer.status === "stale"
          ? "Location updates interrupted"
          : "You're spectating";
  const sharingDescription = isArchive
    ? "This route is closed. Its recorded paths remain available."
    : !liveConnectionReady
      ? "Waiting for the live connection. If it does not connect, reload this page."
      : snapshot.viewer.status === "tracking"
        ? "Keep this page open. Sharing uses battery and mobile data."
        : snapshot.viewer.status === "stale"
          ? "Your last location may be out of date. You can stop sharing below."
          : snapshot.route.sharingPolicy === "joiners_can_view_only" &&
              snapshot.viewer.role !== "owner"
            ? "Only the owner can share location on this route."
            : occupiedSlots >= snapshot.route.maxTrackingMembers
              ? "All tracking slots are in use. You can still follow the group."
              : !snapshot.viewer.canStartSharing
                ? "Location sharing is not available for your membership."
                : "You can see the group without sharing your location.";

  async function handleShareRoute() {
    setIsSharingLink(true);
    setManualShareUrl("");
    const url = new URL(
      `/routes/${encodeURIComponent(code)}`,
      window.location.origin,
    ).href;
    try {
      if (navigator.share) {
        await navigator.share({ title: snapshot.route.name, url });
        return;
      }
      await navigator.clipboard.writeText(url);
      addShareNotice({
        id: ++nextShareNoticeId.current,
        title: "Route link copied",
        detail: "Ready to paste and share",
        tone: "success",
      });
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") return;
      setManualShareUrl(url);
      addShareNotice({
        id: ++nextShareNoticeId.current,
        title: "Copy link manually",
        detail: "Select the link in the route header",
        tone: "error",
      });
    } finally {
      setIsSharingLink(false);
    }
  }

  function addShareNotice(notice: Omit<NotificationItem, "leaving">) {
    setShareNotices((current) =>
      [{ ...notice, leaving: false }, ...current].slice(0, 3),
    );
  }

  useEffect(() => {
    const currentIds = new Set(shareNotices.map((notice) => notice.id));
    for (const [id, timers] of shareNoticeTimers.current) {
      if (currentIds.has(id)) continue;
      timers.forEach(window.clearTimeout);
      shareNoticeTimers.current.delete(id);
    }
    for (const notice of shareNotices) {
      if (shareNoticeTimers.current.has(notice.id)) continue;
      const leave = window.setTimeout(() => {
        setShareNotices((current) =>
          current.map((item) =>
            item.id === notice.id ? { ...item, leaving: true } : item,
          ),
        );
      }, 5750);
      const remove = window.setTimeout(() => {
        setShareNotices((current) =>
          current.filter((item) => item.id !== notice.id),
        );
      }, 6000);
      shareNoticeTimers.current.set(notice.id, [leave, remove]);
    }
  }, [shareNotices]);

  useEffect(() => {
    const timers = shareNoticeTimers.current;
    return () => {
      for (const ids of timers.values()) ids.forEach(window.clearTimeout);
      timers.clear();
    };
  }, []);

  useEffect(() => {
    snapshotRef.current = snapshot;
    setMapState((current) => mergeSnapshotIntoMapState(current, snapshot));
    if (snapshot.viewer.status !== "stale") {
      setShowStaleRecovery(false);
    }
  }, [snapshot]);

  useEffect(() => {
    if (memberToken === "" || snapshot.route.status !== "active") {
      return;
    }

    let connection: RouteLiveSocketConnection;
    connection = connectRouteLiveSocket({
      url: routeWebSocketUrl(),
      memberToken,
      onSocketChange: (socket) => {
        websocketRef.current = socket;
      },
      onDisconnected: () => {
        setLiveConnectionReady(false);
      },
      onMessage: (event) => {
        const liveEvent = parseLiveEvent(event.data);
        if (!liveEvent) {
          return;
        }

        if (liveEvent.type === "position_updated") {
          if (liveEvent.memberId === snapshotRef.current.viewer.memberId) {
            setLiveTrackingError("");
          }
          setMapState((current) =>
            appendLiveRoutePoint(current, {
              memberId: liveEvent.memberId,
              segmentId: liveEvent.segmentId,
              point: liveEvent.point,
            }),
          );
          return;
        }

        if (liveEvent.type === "position_rejected") {
          const viewerStatus = snapshotRef.current.viewer.status;
          if (viewerStatus !== "tracking" && viewerStatus !== "stale") {
            return;
          }
          setLiveTrackingError(positionRejectedMessage(liveEvent.error));
          return;
        }

        if (liveEvent.type === "live_connection_rejected") {
          if (
            connection.isRecovering() &&
            liveEvent.reason === "already_active_connection"
          ) {
            return;
          }

          connection.stop();
          setLiveConnectionRejected(true);
          return;
        }

        if (liveEvent.type === "connection_established") {
          connection.markEstablished();
          setLiveConnectionReady(true);
          return;
        }

        if (liveEvent.type === "route_closed") {
          onSnapshotChange(
            snapshotWithClosedRoute(snapshotRef.current, liveEvent.route),
          );
          return;
        }

        if (liveEvent.type === "command_ack") {
          if (snapshotRef.current.viewer.status !== "stale") {
            setSharingAction(null);
          }
          return;
        }

        if (liveEvent.type === "command_rejected") {
          setSharingAction(null);
          setSharingError(commandRejectedMessage(liveEvent.reason));
          return;
        }

        if (isMemberLiveEvent(liveEvent)) {
          if (liveEvent.member.id === snapshotRef.current.viewer.memberId) {
            setSharingAction(null);
            if (liveEvent.type === "member_stopped_sharing") {
              setLiveTrackingError("");
            }
          }
          const updatedSnapshot = applyMemberLiveEvent(
            snapshotRef.current,
            liveEvent,
          );
          snapshotRef.current = updatedSnapshot;
          onSnapshotChange(updatedSnapshot);
          setMapState((current) =>
            updateMapMemberStatus(
              current,
              liveEvent.member.id,
              liveEvent.member.status,
            ),
          );
        }
      },
    });

    return () => {
      setLiveConnectionReady(false);
      connection.stop();
    };
  }, [memberToken, snapshot.route.status]);

  useEffect(() => {
    if (!isViewerTracking) {
      return;
    }

    setLiveTrackingError("");
    return navigationService.watchPosition(
      (position) => {
        const socket = websocketRef.current;
        if (!socket || socket.readyState !== WebSocket.OPEN) {
          return;
        }

        socket.send(JSON.stringify(positionUpdatePayload(position)));
      },
      (locationError) => {
        setLiveTrackingError(locationErrorMessage(locationError));
      },
    );
  }, [isViewerTracking]);

  async function handleSharingControl() {
    if (!canUseSharingControl) {
      return;
    }

    const shouldStartSharing = !snapshot.viewer.canStopSharing;
    await updateSharing(shouldStartSharing ? "start" : "stop");
  }

  async function handleStaleRecovery(action: "start" | "stop") {
    if (!canUseSharingControl) {
      return;
    }

    await updateSharing(action);
  }

  async function updateSharing(action: "start" | "stop") {
    setSharingError("");
    setSharingAction(action);

    try {
      if (action === "start") {
        await navigationService.requestPermission();
        sendLiveCommand("start_sharing");
      } else {
        sendLiveCommand("stop_sharing");
      }
    } catch (caughtError) {
      setSharingAction(null);
      if (
        caughtError instanceof ApiError &&
        caughtError.code === "unauthorized"
      ) {
        clearRouteAuth(code);
      }

      setSharingError(locationErrorMessage(caughtError));
    }
  }

  async function handleCloseRoute() {
    if (!snapshot.viewer.canCloseRoute || ownerToken === "") {
      return;
    }

    setLifecycleError("");
    setIsLifecycleSubmitting(true);

    try {
      const route = await closeRoute(code, ownerToken);
      onSnapshotChange(snapshotWithClosedRoute(snapshotRef.current, route));
      setLifecycleAction(null);
    } catch (caughtError) {
      setLifecycleError(routeLifecycleErrorMessage(caughtError));
    } finally {
      setIsLifecycleSubmitting(false);
    }
  }

  async function handleDeleteRoute() {
    if (
      !snapshot.viewer.canDeleteRoute ||
      ownerToken === "" ||
      deleteConfirmation.trim().toUpperCase() !== snapshot.route.code
    ) {
      return;
    }

    setLifecycleError("");
    setIsLifecycleSubmitting(true);

    try {
      await deleteRoute(code, ownerToken);
      clearRouteAuth(code);
      onDeleted();
    } catch (caughtError) {
      setLifecycleError(routeLifecycleErrorMessage(caughtError));
      setIsLifecycleSubmitting(false);
    }
  }

  function openLifecycleDialog(action: "close" | "delete") {
    setLifecycleError("");
    setDeleteConfirmation("");
    setLifecycleAction(action);
  }

  function closeLifecycleDialog() {
    if (isLifecycleSubmitting) {
      return;
    }

    setLifecycleAction(null);
    setLifecycleError("");
    setDeleteConfirmation("");
  }

  function sendLiveCommand(type: "start_sharing" | "stop_sharing") {
    const socket = websocketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setSharingAction(null);
      throw new Error("Live route connection is unavailable.");
    }

    socket.send(
      JSON.stringify({
        type,
        requestId: crypto.randomUUID(),
      }),
    );
  }

  if (liveConnectionRejected) {
    return (
      <section className="route-shell">
        <RouteHeader code={code} label="Route" title={snapshot.route.name} />
        <div className="route-panel">
          <p className="form-error" role="alert">
            This route is already open in another tab or device.
          </p>
        </div>
      </section>
    );
  }

  return (
    <section className="route-screen">
      <header className="route-topbar">
        <div className="route-header-identity">
          <img
            className="route-header-logo"
            src="/brand/keepup-mark.svg"
            alt=""
            aria-hidden="true"
            width="54"
            height="54"
          />
          <div className="route-title-block">
            <div className="route-brand-line">
              <span className="route-wordmark" aria-hidden="true">
                keep<span>up</span>
              </span>
              <span className="route-brand-separator" aria-hidden="true">
                /
              </span>
              <p className="eyebrow">
                {isArchive ? "Closed route" : "Live route"}
              </p>
            </div>
            <h1>{snapshot.route.name}</h1>
            <p className="route-code">
              Route code <span>{snapshot.route.code}</span>
            </p>
          </div>
        </div>
        <div className="route-header-actions">
          <button
            className="primary-action compact-action"
            disabled={isSharingLink}
            onClick={handleShareRoute}
            type="button"
          >
            {isSharingLink ? "Sharing..." : "Share"}
          </button>
          <button
            className="secondary-action icon-action"
            onClick={() => setShowSettings(true)}
            type="button"
            aria-haspopup="dialog"
            aria-label="Route details"
          >
            <svg
              aria-hidden="true"
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="currentColor"
            >
              <circle cx="5" cy="12" r="2" />
              <circle cx="12" cy="12" r="2" />
              <circle cx="19" cy="12" r="2" />
            </svg>
          </button>
        </div>
        {manualShareUrl ? (
          <label className="field share-fallback">
            <span>Route link</span>
            <input
              readOnly
              value={manualShareUrl}
              onFocus={(event) => event.target.select()}
            />
          </label>
        ) : null}
      </header>

      <NotificationStack items={shareNotices} />

      <RouteMap
        state={mapState}
        archived={isArchive}
        sharingCount={sharingCount}
      />

      {showStaleRecovery ? (
        <Modal
          describedBy="stale-recovery-description"
          labelledBy="stale-recovery-title"
        >
          <p className="eyebrow">Sharing interrupted</p>
          <h2 id="stale-recovery-title">Continue sharing your location?</h2>
          <p id="stale-recovery-description">
            Your previous sharing session was interrupted. Choose how to
            continue on this route.
          </p>

          <div className="recovery-actions">
            <button
              className="primary-action"
              disabled={!canUseSharingControl}
              onClick={() => handleStaleRecovery("start")}
              type="button"
            >
              {sharingAction === "start" ? "Resuming..." : "Resume sharing"}
            </button>
            <button
              className="secondary-action"
              disabled={!canUseSharingControl}
              onClick={() => handleStaleRecovery("stop")}
              type="button"
            >
              {sharingAction === "stop"
                ? "Continuing..."
                : "Continue as spectator"}
            </button>
          </div>

          {!liveConnectionReady && !sharingError && !liveTrackingError ? (
            <p className="route-status">Connecting to the live route...</p>
          ) : null}

          {sharingError ? (
            <p className="form-error" role="alert">
              {sharingError}
            </p>
          ) : null}

          {!sharingError && liveTrackingError ? (
            <p className="form-error" role="alert">
              {liveTrackingError}
            </p>
          ) : null}
        </Modal>
      ) : null}

      {lifecycleAction ? (
        <Modal
          describedBy="route-lifecycle-description"
          labelledBy="route-lifecycle-title"
          onClose={closeLifecycleDialog}
        >
          <p className="eyebrow">Owner action</p>
          <h2 id="route-lifecycle-title">
            {lifecycleAction === "close"
              ? "Close this route?"
              : "Permanently delete this route?"}
          </h2>
          <p id="route-lifecycle-description">
            {lifecycleAction === "close"
              ? "Closing stops all location sharing and turns this route into a read-only archive. It cannot be reopened."
              : "Deleting permanently removes the route, members, and all recorded paths. This cannot be undone."}
          </p>

          {lifecycleAction === "delete" ? (
            <label className="field">
              <span>
                Type <strong>{snapshot.route.code}</strong> to confirm
              </span>
              <input
                autoComplete="off"
                disabled={isLifecycleSubmitting}
                onChange={(event) => setDeleteConfirmation(event.target.value)}
                value={deleteConfirmation}
              />
            </label>
          ) : null}

          <div className="recovery-actions">
            <button
              className="danger-action"
              disabled={
                isLifecycleSubmitting ||
                (lifecycleAction === "delete" &&
                  deleteConfirmation.trim().toUpperCase() !==
                    snapshot.route.code)
              }
              onClick={
                lifecycleAction === "close"
                  ? handleCloseRoute
                  : handleDeleteRoute
              }
              type="button"
            >
              {isLifecycleSubmitting
                ? lifecycleAction === "close"
                  ? "Closing..."
                  : "Deleting..."
                : lifecycleAction === "close"
                  ? "Close route"
                  : "Delete route permanently"}
            </button>
            <button
              className="secondary-action"
              disabled={isLifecycleSubmitting}
              onClick={closeLifecycleDialog}
              data-initial-focus
              type="button"
            >
              Cancel
            </button>
          </div>

          {lifecycleError ? (
            <p className="form-error" role="alert">
              {lifecycleError}
            </p>
          ) : null}
        </Modal>
      ) : null}

      {showSettings ? (
        <Modal
          labelledBy="route-details-title"
          onClose={() => setShowSettings(false)}
        >
          <div className="sheet-heading">
            <h2 id="route-details-title">Route details</h2>
            <button
              className="text-action"
              autoFocus
              onClick={() => setShowSettings(false)}
              type="button"
            >
              Done
            </button>
          </div>
          <p className="route-description">
            {snapshot.route.description || "No description for this route."}
          </p>
          <div className="detail-list">
            <p>
              <span>Sharing</span>
              <strong>
                {snapshot.route.sharingPolicy === "everyone_can_share"
                  ? "Everyone can share"
                  : "Only the owner can share"}
              </strong>
            </p>
            <p>
              <span>Access</span>
              <strong>
                {snapshot.route.hasPassword
                  ? "Password protected"
                  : "Anyone with the link"}
              </strong>
            </p>
            <p>
              <span>Route code</span>
              <strong>{snapshot.route.code}</strong>
            </p>
            <p>
              <span>Your role</span>
              <strong>{formatRole(snapshot.viewer.role)}</strong>
            </p>
          </div>
          {snapshot.viewer.canCloseRoute || snapshot.viewer.canDeleteRoute ? (
            <div className="management-actions">
              <h3>Manage route</h3>
              {ownerToken === "" ? (
                <p className="route-status">
                  Owner access is unavailable in this browser.
                </p>
              ) : null}
              {snapshot.viewer.canCloseRoute ? (
                <button
                  className="secondary-action"
                  disabled={ownerToken === ""}
                  onClick={() => openLifecycleDialog("close")}
                  type="button"
                >
                  Close route
                </button>
              ) : null}
              {snapshot.viewer.canDeleteRoute ? (
                <button
                  className="danger-action danger-action-subtle"
                  disabled={ownerToken === ""}
                  onClick={() => openLifecycleDialog("delete")}
                  type="button"
                >
                  Delete route
                </button>
              ) : null}
            </div>
          ) : null}
        </Modal>
      ) : null}

      <aside className="member-sheet" aria-label="Sharing and route members">
        <div className="sheet-handle" aria-hidden="true" />
        <div className="sheet-section sharing-section">
          <div className="sheet-heading">
            <h2>{sharingTitle}</h2>
          </div>
          <p className="route-status" id="sharing-description">
            {sharingDescription}
          </p>
          {!isArchive ? (
            <button
              className={
                snapshot.viewer.canStopSharing
                  ? "secondary-action"
                  : "sharing-action"
              }
              disabled={!canUseSharingControl}
              onClick={handleSharingControl}
              aria-describedby="sharing-description"
              type="button"
            >
              {sharingAction ? sharingControlBusyLabel : sharingControlLabel}
            </button>
          ) : null}
          {sharingError ? (
            <p className="form-error" role="alert">
              {sharingError}
            </p>
          ) : null}
          {liveTrackingError && !isArchive ? (
            <p className="form-error" role="alert">
              {liveTrackingError}
            </p>
          ) : null}
        </div>
        <div className="sheet-section">
          <div className="sheet-heading">
            <h2>
              Members{" "}
              <span className="member-count">{sortedMembers.length}</span>
            </h2>
            <button
              className="text-action"
              type="button"
              aria-expanded={membersExpanded}
              aria-controls="route-members"
              onClick={() => setMembersExpanded(!membersExpanded)}
            >
              {membersExpanded ? "Collapse" : "Expand"}
            </button>
          </div>
          <div
            className="member-list"
            id="route-members"
            role="region"
            aria-label="Route member list"
            tabIndex={0}
            hidden={!membersExpanded}
          >
            {sortedMembers.map((member) => (
              <MemberRow
                key={member.id}
                member={member}
                isViewer={member.id === snapshot.viewer.memberId}
              />
            ))}
          </div>
        </div>
      </aside>
    </section>
  );
}

function MemberRow({
  member,
  isViewer,
}: {
  member: SnapshotMember;
  isViewer: boolean;
}) {
  const initials = Array.from(member.displayName.trim())
    .slice(0, 2)
    .join("")
    .toUpperCase();
  return (
    <article className={`member-row member-${member.status}`}>
      <span
        aria-hidden="true"
        className="member-avatar"
        style={{ borderColor: member.color }}
      >
        {initials}
        <span
          className="member-color"
          style={{ backgroundColor: member.color }}
        />
      </span>
      <div className="member-info">
        <h3>
          {member.displayName}
          {isViewer ? <small> · You</small> : null}
        </h3>
        <p>
          {member.role === "owner" ? "Owner · " : ""}
          {formatTransportMode(member.transportMode)}
        </p>
        <StatusBadge status={member.status} />
      </div>
    </article>
  );
}

function snapshotWithClosedRoute(
  snapshot: RouteSnapshot,
  route: RouteSummary,
): RouteSnapshot {
  return {
    ...snapshot,
    route,
    members: snapshot.members.map((member) => ({
      ...member,
      status:
        member.status === "tracking" || member.status === "stale"
          ? "spectating"
          : member.status,
    })),
    viewer: {
      ...snapshot.viewer,
      status:
        snapshot.viewer.status === "tracking" ||
        snapshot.viewer.status === "stale"
          ? "spectating"
          : snapshot.viewer.status,
      canStartSharing: false,
      canStopSharing: false,
      canCloseRoute: false,
    },
  };
}

function positionUpdatePayload(position: NavigationPosition) {
  return {
    type: "position_update",
    latitude: position.latitude,
    longitude: position.longitude,
    accuracyM: position.accuracyM,
    altitudeM: position.altitudeM,
    speedMps: position.speedMps,
    headingDeg: position.headingDeg,
    clientRecordedAt: position.clientRecordedAt,
  };
}

function positionRejectedMessage(error?: string) {
  switch (error) {
    case "accuracy_required":
      return "Your device did not report location accuracy. Waiting for another location reading.";
    case "accuracy_too_low":
      return "Your location is too imprecise. Move to an open area if possible. Sharing will recover when accuracy improves.";
    case "timestamp_required":
      return "Your device did not report when this location was recorded. Waiting for another location reading.";
    case "timestamp_too_old":
      return "This location reading is too old. Waiting for a fresh reading. If this continues, check that your device clock is set automatically.";
    case "timestamp_in_future":
      return "This location reading has a future timestamp. Check that your device clock is set automatically.";
    case "duplicate_timestamp":
      return "Your device sent the same location timestamp again. Waiting for a new reading.";
    case "out_of_order_timestamp":
      return "This location reading arrived out of order. Waiting for a newer reading.";
    case "impossible_jump":
      return "This location reading shows an unusually large jump. Keeping your last accepted location while waiting for another reading.";
  }

  if (error === "route_closed") {
    return "This route is closed.";
  }

  if (error === "unauthorized") {
    return "Route access expired. Join again to continue.";
  }

  if (error === "invalid_input") {
    return "The latest location update could not be used.";
  }

  return "The latest location update was rejected.";
}

function commandRejectedMessage(reason?: string) {
  if (reason === "tracking_limit_reached") {
    return "All tracking slots are currently in use.";
  }
  if (reason === "sharing_not_allowed") {
    return "This route only allows the owner to share location.";
  }
  if (reason === "route_closed") {
    return "This route is closed.";
  }

  return "Could not update sharing.";
}

function locationErrorMessage(error: unknown) {
  if (error instanceof Error) {
    if (error.message === "geolocation_denied") {
      return "Location permission is blocked. Allow location access for this site in your browser settings, then try again.";
    }
    if (error.message === "geolocation_unavailable") {
      return "Your device could not determine its location. Check location services and try again.";
    }
    if (error.message === "geolocation_timeout") {
      return "Getting your location timed out. Move somewhere with a clearer signal and try again.";
    }
    if (error.message === "geolocation_failed") {
      return "Could not get your location. Check browser and device location settings, then try again.";
    }

    return error.message;
  }

  return "Could not update sharing.";
}

function routeLifecycleErrorMessage(error: unknown) {
  if (error instanceof ApiError && error.code === "unauthorized") {
    return "Owner access expired. This route can no longer be managed from this browser.";
  }

  return error instanceof Error
    ? error.message
    : "Could not update this route.";
}

function compareMembers(first: SnapshotMember, second: SnapshotMember) {
  const statusOrder: Record<string, number> = {
    tracking: 1,
    stale: 2,
    spectating: 3,
    offline: 4,
    left: 5,
  };

  if (first.role !== second.role) {
    return first.role === "owner" ? -1 : 1;
  }

  const firstStatus = statusOrder[first.status] ?? 99;
  const secondStatus = statusOrder[second.status] ?? 99;
  if (firstStatus !== secondStatus) {
    return firstStatus - secondStatus;
  }

  return (
    new Date(first.joinedAt).getTime() - new Date(second.joinedAt).getTime()
  );
}

function formatRole(role: string) {
  return role === "owner" ? "Owner" : "Member";
}

function formatStatus(status: string) {
  return status
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function formatTransportMode(mode: string) {
  return transportLabels[mode as TransportMode] ?? formatStatus(mode);
}

function RouteHeader({
  code,
  label,
  title,
}: {
  code: string;
  label: string;
  title?: string;
}) {
  return (
    <div className="form-header">
      <Brand />
      <p className="eyebrow">{label}</p>
      <h1>{title || code}</h1>
      <p className="route-code">
        Route code <span>{code}</span>
      </p>
    </div>
  );
}
