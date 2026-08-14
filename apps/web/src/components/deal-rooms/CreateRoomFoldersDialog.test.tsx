// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { CreateRoomFoldersDialog } from "./CreateRoomFoldersDialog";

async function renderDialog(
  onConfirm = vi.fn(),
  folders: { name: string; path?: string }[] = [{ name: "Pitch", path: "/pitch-deck" }],
) {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        dealRooms: {
          new: {
            cancel: "Cancel",
            creating: "Creating...",
            foldersDialog: {
              title: "Review folders",
              subtitle: "{{name}} — keep what you need, rename in the list, add a folder or one subfolder under it.",
              add: "Add folder",
              addSubfolder: "Add subfolder",
              addPlaceholder: "Folder name",
              rename: "Rename",
              select: "Keep {{name}}",
              empty: "Add a folder to continue.",
              create: "Create",
            },
          },
        },
      },
    },
  });
  render(
    <I18nextProvider i18n={instance}>
      <CreateRoomFoldersDialog
        open
        onOpenChange={() => undefined}
        templateName="Startup Fundraising"
        folders={folders}
        creating={false}
        onConfirm={onConfirm}
      />
    </I18nextProvider>,
  );
  return onConfirm;
}

describe("CreateRoomFoldersDialog", () => {
  it("exposes rename and add actions, and creates a two-level payload", async () => {
    const onConfirm = await renderDialog();

    expect(screen.getByRole("button", { name: /^rename$/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^rename$/i }));
    fireEvent.change(screen.getByLabelText("Folder name"), { target: { value: "Deck" } });
    fireEvent.blur(screen.getByLabelText("Folder name"));

    fireEvent.click(screen.getByRole("button", { name: /add subfolder/i }));
    fireEvent.change(screen.getByLabelText("Folder name"), { target: { value: "Annex" } });
    fireEvent.blur(screen.getByLabelText("Folder name"));

    fireEvent.click(screen.getByRole("button", { name: /add folder/i }));
    fireEvent.change(screen.getByLabelText("Folder name"), { target: { value: "Legal" } });
    fireEvent.blur(screen.getByLabelText("Folder name"));

    fireEvent.click(screen.getByTestId("confirm-create-room"));
    expect(onConfirm).toHaveBeenCalledWith([
      { name: "Deck", path: "/deck" },
      { name: "Annex", path: "/deck/annex" },
      { name: "Legal", path: "/legal" },
    ]);
  });
});
