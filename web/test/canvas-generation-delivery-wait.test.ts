import { beforeEach, describe, expect, test } from "bun:test";

import { bindBackendCanvasGenerationResult, CanvasGenerationDurableAckError } from "@/services/canvas-generation-consumer";
import { ApiError } from "@/services/api/request";
import { useCanvasStore, type CanvasProject } from "@/stores/canvas/use-canvas-store";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";
import type { GenerationTask } from "@/services/api/task-center";

function captured(userScope: string, epoch: number) {
    return { userScope, epoch };
}

function canvasNode(id: string, metadata: CanvasNodeData["metadata"]): CanvasNodeData {
    return { id, type: CanvasNodeType.Image, title: "镜头", position: { x: 1, y: 1 }, width: 320, height: 220, metadata };
}

function canvasProject(id: string, nodes: CanvasNodeData[], revision = 4): CanvasProject {
    return {
        id,
        revision,
        title: "画布",
        createdAt: "2026-01-01T00:00:00.000Z",
        updatedAt: "2026-01-01T00:00:00.000Z",
        nodes,
        connections: [],
        chatSessions: [],
        activeChatId: null,
        backgroundMode: "grid",
        showImageInfo: false,
        viewport: { x: 0, y: 0, k: 1 },
        directorScenes: [],
    };
}

function mediaTask(overrides: Partial<GenerationTask> = {}): GenerationTask {
    return {
        id: "task-1",
        type: "canvas_image",
        status: "succeeded",
        prompt: "猫",
        attempts: 1,
        createdAt: "2026-01-01T00:00:00.000Z",
        updatedAt: "2026-01-01T00:00:00.000Z",
        ...overrides,
    };
}

function bindResponse(operationId: string, project: CanvasProject, nodeId: string) {
    return {
        op: "canvas.task.bind",
        opId: operationId,
        replayed: false,
        revision: project.revision,
        result: {
            applied: true,
            canvasId: project.id,
            nodeId,
            taskId: "task-1",
            bindingStatus: "bound" as const,
            content: "/api/resources/res-1/file",
            storageKey: "resource:res-1",
            assetId: "generation_abc",
            revision: project.revision,
            canvas: project,
            node: project.nodes.find((item) => item.id === nodeId),
        },
    };
}

// Mirrors use-canvas-generation.applyGenerationTaskResult's wrapping so the test
// observes the same error code the user sees.
async function applyLikeCaller(input: Parameters<typeof bindBackendCanvasGenerationResult>[0]) {
    try {
        await bindBackendCanvasGenerationResult(input);
    } catch (error) {
        if (error instanceof Error && error.name === "AbortError") throw error;
        throw error instanceof CanvasGenerationDurableAckError ? error : new CanvasGenerationDurableAckError(error);
    }
}

describe("bindBackendCanvasGenerationResult waits for delivery", () => {
    beforeEach(() => {
        useCanvasStore.setState({ projects: [] });
    });

    test("waits for materialized output instead of surfacing a canvas conflict", async () => {
        const loading = canvasNode("node-1", { taskId: "task-1", status: "loading" });
        const project = canvasProject("canvas-1", [{ ...loading, metadata: { ...loading.metadata, status: "success", content: "/api/resources/res-1/file" } }], 4);
        useCanvasStore.setState({ projects: [canvasProject("canvas-1", [loading], 3)] });
        const nodesRef = { current: [loading] };
        const identity = captured("user-a", 1);

        const pendingTask = mediaTask({ outputs: [{ outputIndex: 0, mediaType: "image" }] });
        const deliveredTask = mediaTask({ outputs: [{ outputIndex: 0, mediaType: "image", materializedAssetId: "generation_abc" }] });

        let refreshes = 0;
        let bindCalls = 0;
        let caught: unknown;
        try {
            await applyLikeCaller({
                canvasId: "canvas-1",
                nodeId: "node-1",
                task: pendingTask,
                isCurrent: () => true,
                nodesRef,
                setNodes: (value) => {
                    nodesRef.current = typeof value === "function" ? value(nodesRef.current) : value;
                },
                runtime: {
                    hydrateOutputs: async () => undefined,
                    persistDocument: async () => undefined,
                    bindOutput: async (input) => {
                        bindCalls += 1;
                        return bindResponse(input.operationId, project, "node-1");
                    },
                    adoptConfirmedProjection: async (adopted) => adopted,
                    captureScope: () => identity,
                    liveScope: () => identity,
                    refreshTask: async () => {
                        refreshes += 1;
                        return deliveredTask;
                    },
                    sleep: async () => undefined,
                    now: () => Date.now(),
                },
            });
        } catch (error) {
            caught = error;
        }

        expect(caught).toBeUndefined();
        expect(refreshes).toBeGreaterThanOrEqual(1);
        expect(bindCalls).toBe(1);
        expect(nodesRef.current[0]?.metadata?.status).toBe("success");
    });

    test("retries bind after output_not_ready once the output materializes", async () => {
        const loading = canvasNode("node-1", { taskId: "task-1", status: "loading" });
        const project = canvasProject("canvas-1", [{ ...loading, metadata: { ...loading.metadata, status: "success", content: "/api/resources/res-1/file" } }], 4);
        useCanvasStore.setState({ projects: [canvasProject("canvas-1", [loading], 3)] });
        const nodesRef = { current: [loading] };
        const identity = captured("user-a", 1);

        const deliveredTask = mediaTask({ outputs: [{ outputIndex: 0, mediaType: "image", materializedAssetId: "generation_abc" }] });
        let refreshes = 0;
        let bindCalls = 0;

        await applyLikeCaller({
            canvasId: "canvas-1",
            nodeId: "node-1",
            task: deliveredTask,
            isCurrent: () => true,
            nodesRef,
            setNodes: (value) => {
                nodesRef.current = typeof value === "function" ? value(nodesRef.current) : value;
            },
            runtime: {
                hydrateOutputs: async () => undefined,
                persistDocument: async () => undefined,
                bindOutput: async (input) => {
                    bindCalls += 1;
                    if (bindCalls === 1) throw new ApiError("任务产物尚未交付，不能绑定到画布", { status: 412, reason: "output_not_ready" });
                    return bindResponse(input.operationId, project, "node-1");
                },
                adoptConfirmedProjection: async (adopted) => adopted,
                captureScope: () => identity,
                liveScope: () => identity,
                refreshTask: async () => {
                    refreshes += 1;
                    return deliveredTask;
                },
                sleep: async () => undefined,
                now: () => Date.now(),
            },
        });

        expect(refreshes).toBeGreaterThanOrEqual(1);
        expect(bindCalls).toBe(2);
    });
});
