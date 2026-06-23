class CreateMachines < ActiveRecord::Migration[8.1]
  def change
    create_table :machines do |t|
      t.string :name
      t.string :secret_key

      t.timestamps
    end
    add_index :machines, :name, unique: true
  end
end
