# Simple Save Manager (legacy C# version)

> [!NOTE]
> This branch keeps the original C# console app for history.
> It was rewritten from scratch as **[Bonfire](https://github.com/rsxcore/Bonfire)**: see the `main` branch.

A small console tool that backs up and restores saves for Dark Souls (PtDE, Remastered, II, III), Sekiro, Elden Ring and Bloodborne (shadPS4).

![Game selection](https://github.com/user-attachments/assets/3f289d61-14de-4c4c-abcb-982840947e60)

![Save menu](https://github.com/user-attachments/assets/bf00eee1-217d-4693-acc5-529e6bc28e9a)

## Build

Requires the [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0).

```powershell
dotnet run --project "save manager"
```

Backups are stored in `Desktop\backup\<Game>_save\`.
