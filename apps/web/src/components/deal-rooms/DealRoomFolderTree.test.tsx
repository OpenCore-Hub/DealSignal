// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DealRoomFolderTree } from "./DealRoomFolderTree";
import { DealRoomDocumentsHome } from "./DealRoomDocumentsHome";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";
import type { DealRoomFolder, DealRoomFolderDocs } from "@/types";
import { toast } from "sonner";

vi.mock("@/lib/api", () => ({
  api: {
    lockDealRoomResources: vi.fn().mockResolvedValue(undefined),
    unlockDealRoomResources: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const i18nInstance = i18n.createInstance();
void i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: enDealRooms,
      common: { loading: "Loading...", cancel: "Cancel", save: "Save", delete: "Delete" },
      documents: { upload: { supportedTypes: "*" } },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

const folders: DealRoomFolder[] = [
  { path: "/legal", name: "Legal", sort_order: 0 },
  { path: "/finance", name: "Finance", sort_order: 1, locked: true },
];

const folderDocs: DealRoomFolderDocs[] = [
  {
    folder: "/legal",
    permission: "admin",
    documents: [
      {
        id: "rd-1",
        document_id: "doc-1",
        title: "Board Minutes",
        folder_path: "/legal",
        sort_order: 0,
        source_type: "pdf",
        status: "ready",
        created_at: "2026-01-01T00:00:00Z",
      },
    ],
  },
  { folder: "/finance", permission: "admin", documents: [] },
];

describe("DealRoomFolderTree toolbar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders search and filters, and switches to bulk actions on select", () => {
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          isAdmin
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
          onDocumentRemove={async () => {}}
          onFolderUpload={async () => {}}
        />
      </Wrapper>,
    );

    const toolbar = screen.getByTestId("folder-tree-toolbar");
    expect(toolbar).toBeInTheDocument();
    const search = screen.getByLabelText(/search folders and files/i);
    expect(search).toBeInTheDocument();
    // Order: search → create directory → batch lock → batch unlock.
    expect(toolbar).toHaveTextContent("Create directory");
    expect(toolbar).toHaveTextContent("Batch lock");
    expect(toolbar).toHaveTextContent("Batch unlock");
    expect(toolbar.firstElementChild).toContainElement(search);
    expect(screen.queryByLabelText(/lock status/i)).not.toBeInTheDocument();

    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes.length).toBeGreaterThan(0);
    fireEvent.click(checkboxes[0]!);

    // Idle chrome (search / create directory) hides; selection actions take over.
    expect(screen.queryByLabelText(/search folders and files/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-create-directory")).not.toBeInTheDocument();
    // Legal folder + its document are selected together.
    expect(screen.getByText(/2 selected/i)).toBeInTheDocument();
    expect(toolbar).toHaveTextContent("Create subdirectory");
    expect(toolbar).toHaveTextContent("Delete directory");
    expect(toolbar).toHaveTextContent("Batch upload");
    expect(toolbar).toHaveTextContent("Remove files");
    expect(toolbar).toHaveTextContent("Batch lock");
    expect(toolbar).toHaveTextContent("Batch unlock");
    const removeBtn = screen.getByTestId("folder-tree-bulk-remove-files");
    const lockBtn = screen.getByTestId("folder-tree-bulk-lock");
    const unlockBtn = screen.getByTestId("folder-tree-bulk-unlock");
    expect(
      removeBtn.compareDocumentPosition(lockBtn) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      lockBtn.compareDocumentPosition(unlockBtn) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(toolbar).not.toHaveTextContent(/^Clear$/);
  });

  it("portals the toolbar into the documents chrome host", () => {
    render(
      <Wrapper>
        <DealRoomDocumentsHome
          activeLinkCount={0}
          failedDeliveries={0}
          unreadQuestions={0}
          onJumpTab={() => {}}
        >
          <DealRoomFolderTree
            roomId="room-1"
            folders={folders}
            folderDocs={folderDocs}
            isAdmin
            onFolderCreate={async () => {}}
            onFolderRename={async () => {}}
            onFolderDelete={async () => {}}
          />
        </DealRoomDocumentsHome>
      </Wrapper>,
    );

    const host = screen.getByTestId("deal-room-resources-toolbar-host");
    const toolbar = screen.getByTestId("folder-tree-toolbar");
    expect(host).toContainElement(toolbar);
    expect(screen.queryByTestId("deal-room-attention-banner")).not.toBeInTheDocument();
    expect(screen.getByTestId("folder-tree-create-directory")).toBeInTheDocument();
  });

  it("starts a root directory create row from the toolbar", async () => {
    const onFolderCreate = vi.fn().mockResolvedValue(undefined);
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          isAdmin
          onFolderCreate={onFolderCreate}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
        />
      </Wrapper>,
    );

    fireEvent.click(screen.getByTestId("folder-tree-create-directory"));
    const input = screen.getByPlaceholderText(/folder name/i);
    fireEvent.change(input, { target: { value: "New Diligence" } });
    fireEvent.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(onFolderCreate).toHaveBeenCalledWith("New Diligence", "/");
    });
  });

  it("removes selected files from the room via the selection toolbar", async () => {
    const onDocumentRemove = vi.fn().mockResolvedValue(undefined);
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          isAdmin
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
          onDocumentRemove={onDocumentRemove}
          onFolderUpload={async () => {}}
        />
      </Wrapper>,
    );

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[0]!); // Legal + Board Minutes
    fireEvent.click(screen.getByTestId("folder-tree-bulk-remove-files"));

    await waitFor(() => {
      expect(onDocumentRemove).toHaveBeenCalledWith("doc-1");
    });
    confirmSpy.mockRestore();
  });

  it("filters folders by search query", () => {
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          isAdmin
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
        />
      </Wrapper>,
    );

    fireEvent.change(screen.getByLabelText(/search folders and files/i), {
      target: { value: "Finance" },
    });
    expect(screen.getByText("Finance")).toBeInTheDocument();
    expect(screen.queryByText("Legal")).not.toBeInTheDocument();
  });

  it("selecting a folder recursively selects nested folders and documents", () => {
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={[
            ...folders,
            { path: "/legal/nda", name: "NDA", sort_order: 0 },
          ]}
          folderDocs={[
            ...folderDocs,
            {
              folder: "/legal/nda",
              permission: "admin",
              documents: [
                {
                  id: "rd-2",
                  document_id: "doc-2",
                  title: "Standard NDA",
                  folder_path: "/legal/nda",
                  sort_order: 0,
                  source_type: "pdf",
                  status: "ready",
                  created_at: "2026-01-01T00:00:00Z",
                },
              ],
            },
          ]}
          isAdmin
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
        />
      </Wrapper>,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "Legal" }));
    // Legal + NDA folder + Board Minutes + Standard NDA
    expect(screen.getByText(/4 selected/i)).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Legal" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: "NDA" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: "Board Minutes" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("checkbox", { name: "Standard NDA" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("selects a document without opening it when the checkbox is clicked", () => {
    const onDocumentOpen = vi.fn();
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          isAdmin
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
          onDocumentOpen={onDocumentOpen}
        />
      </Wrapper>,
    );

    // Folders start expanded; clicking the doc checkbox must select, not open.
    const docCheckbox = screen.getByRole("checkbox", { name: "Board Minutes" });
    fireEvent.click(docCheckbox);

    // Sole document under Legal also syncs the parent folder key.
    expect(screen.getByText(/2 selected/i)).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Legal" })).toHaveAttribute("aria-checked", "true");
    expect(onDocumentOpen).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /Board Minutes/i }));
    expect(onDocumentOpen).toHaveBeenCalledWith("doc-1");
  });

  it("lets contributors select and upload without manage chrome", () => {
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          canContribute
          canManage={false}
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
          onDocumentRemove={async () => {}}
          onFolderUpload={async () => {}}
        />
      </Wrapper>,
    );

    const toolbar = screen.getByTestId("folder-tree-toolbar");
    expect(screen.getByLabelText(/search folders and files/i)).toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-create-directory")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-lock")).not.toBeInTheDocument();
    expect(screen.getAllByRole("checkbox").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("checkbox", { name: "Legal" }));
    expect(screen.getByText(/2 selected/i)).toBeInTheDocument();
    expect(screen.getByTestId("folder-tree-bulk-upload")).toBeInTheDocument();
    expect(toolbar).toHaveTextContent("Batch upload");
    expect(screen.queryByTestId("folder-tree-bulk-create-subfolder")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-delete-directory")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-remove-files")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-lock")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-unlock")).not.toBeInTheDocument();
  });

  it("keeps search for oversight without checkboxes or mutate actions", () => {
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
          onDocumentRemove={async () => {}}
          onFolderUpload={async () => {}}
        />
      </Wrapper>,
    );

    expect(screen.getByLabelText(/search folders and files/i)).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-create-directory")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-upload")).not.toBeInTheDocument();
  });

  it("continues bulk upload after a per-file error", async () => {
    const onFolderUpload = vi
      .fn()
      .mockRejectedValueOnce(new Error("first failed"))
      .mockResolvedValueOnce(undefined);
    render(
      <Wrapper>
        <DealRoomFolderTree
          roomId="room-1"
          folders={folders}
          folderDocs={folderDocs}
          isAdmin
          onFolderCreate={async () => {}}
          onFolderRename={async () => {}}
          onFolderDelete={async () => {}}
          onDocumentRemove={async () => {}}
          onFolderUpload={onFolderUpload}
        />
      </Wrapper>,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "Legal" }));
    fireEvent.click(screen.getByTestId("folder-tree-bulk-upload"));

    const input = screen.getByTestId("folder-tree-bulk-upload-input");
    const first = new File(["a"], "a.pdf", { type: "application/pdf" });
    const second = new File(["b"], "b.pdf", { type: "application/pdf" });
    await act(async () => {
      Object.defineProperty(input, "files", {
        configurable: true,
        value: [first, second],
      });
      fireEvent.change(input);
    });

    await waitFor(() => {
      expect(onFolderUpload).toHaveBeenCalledTimes(2);
    });
    expect(onFolderUpload).toHaveBeenNthCalledWith(1, first, "/legal", 1);
    expect(onFolderUpload).toHaveBeenNthCalledWith(2, second, "/legal", 2);
    expect(toast.success).toHaveBeenCalled();
  });
});
