using SoulsSaveManager.Tools;
using System.Drawing;

namespace SoulsSaveManager
{
    internal static class Program
    {
        private static void Main(string[] args)
        {
            Initialize();

            while (true)
            {
                SaveManager saveManager = null!;

                #region StartingProgram
                ConsoleUtility.WriteBox(["?>"], "Select supported games");
                ConsoleUtility.WriteBox(SaveManager.SoulsGameList.Select((game, i) => $"$> [{i + 1}]> {game}"));

                // Put the cursor after the "?>" prompt: back over the game list box and the header's bottom line.
                Console.SetCursorPosition(4, Console.CursorTop - (SaveManager.SoulsGameList.Length + 2) - 2);
                #endregion

                ConsoleKeyInfo keyInfo = Console.ReadKey(true);
                Console.Clear();

                switch (keyInfo.Key)
                {
                    case ConsoleKey.D1:
                        saveManager = new SaveManager("Dark Souls: Prepare To Die Edition", WorkingPaths.DarkSoulsPreparePath, "NBGI/DarkSouls", "DarkSoulsPrepareToDieEdition_save");
                        break;

                    case ConsoleKey.D2:
                        saveManager = new SaveManager("Dark Souls: Remastered", WorkingPaths.DarkSoulsRemasteredPath, "NBGI/DARK SOULS REMASTERED", "DarkSoulsRemastered_save");
                        break;

                    case ConsoleKey.D3:
                        saveManager = new SaveManager("Dark Souls II", WorkingPaths.DarkSoulsIIPath, "DarkSoulsII", "DarkSoulsII_save");
                        break;

                    case ConsoleKey.D4:
                        saveManager = new SaveManager("Dark Souls III", WorkingPaths.DarkSoulsIIIPath, "DarkSoulsIII", "DarkSoulsIII_save");
                        break;

                    case ConsoleKey.D5:
                        saveManager = new SaveManager("Sekiro: Shadows Die Twice", WorkingPaths.SekiroPath, "Sekiro", "SekiroShadowsDieTwice_save");
                        break;

                    case ConsoleKey.D6:
                        saveManager = new SaveManager("Elden Ring", WorkingPaths.EldenRingPath, "EldenRing", "EldenRing_save");
                        break;
                    case ConsoleKey.D7:
                        string? bloodbornePath = GetBloodbornePath();
                        if (bloodbornePath == null)
                            continue;

                        WorkingPaths.BloodbornePath = bloodbornePath;
                        saveManager = new SaveManager("Bloodborne", WorkingPaths.BloodbornePath, Path.GetFileName(bloodbornePath), "Bloodborne_save");
                        break;
                    default: continue;

                }

                Console.Title = saveManager.GameName;

                saveManager.Invoke();
            }
        }

        // Asks for the shadPS4 folder once and remembers it. Returns null if no Bloodborne save is found there.
        private static string? GetBloodbornePath()
        {
            string? emulatorPath = File.Exists(WorkingPaths.ConfigPath)
                ? File.ReadAllText(WorkingPaths.ConfigPath).Trim()
                : null;

            if (string.IsNullOrEmpty(emulatorPath))
            {
                Console.Write("[!]> Specify the path to ShadPS4 emulator folder: ");
                emulatorPath = Console.ReadLine()?.Trim().Trim('"') ?? "";
                Console.Clear();
            }

            string? savePath = WorkingPaths.FindBloodborneSave(emulatorPath);
            if (savePath == null)
            {
                // Forget the folder so the user is asked again next time.
                File.Delete(WorkingPaths.ConfigPath);
                Colorful.Console.WriteLine($" [!]> No Bloodborne save found in \"{emulatorPath}\"", Color.Red);
                return null;
            }

            File.WriteAllText(WorkingPaths.ConfigPath, emulatorPath);
            return savePath;
        }

        private static void Initialize()
        {
            Colorful.Console.ForegroundColor = Color.FromArgb(255, 151, 124, 163);
            Console.Title = "From Software Games Save Manager";
        }
    }
}
